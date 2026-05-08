package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	protomodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	pmodel "github.com/prometheus/common/model"
	"golang.org/x/oauth2"

	"github.com/squat/llama-swap-exporter/version"
)

func main() {
	upstreams := flag.String("upstream", "", "Comma-separated llama-swap base URLs")
	apiKey := flag.String("api-key", "", "Bearer token for llama-swap auth")
	metricsPath := flag.String("metrics-path", "/metrics", "HTTP path on which to serve metrics")
	listenAddr := flag.String("web.listen-address", ":9293", "Address on which to serve metrics")
	scrapeTimeout := flag.Duration("scrape.timeout", 10*time.Second, "Per-target scrape timeout")
	v := flag.Bool("version", false, "Print the version of llama-swap-exporter and exit")
	flag.Parse()

	if *v {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	logger := slog.Default()

	if *upstreams == "" {
		logger.Error("--upstream is required")
		os.Exit(1)
	}

	var upstreamURLs []*url.URL
	for u := range strings.SplitSeq(*upstreams, ",") {
		uu, err := url.Parse(u)
		if err != nil {
			logger.Error("upstream is invalid", "upstream", u, "err", err)
			os.Exit(1)
		}
		upstreamURLs = append(upstreamURLs, uu)
	}

	ctx := context.Background()
	collector := newLlamaSwapCollector(ctx, logger, upstreamURLs, *apiKey, *scrapeTimeout)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte(`<html>
<head><title>llama-swap exporter</title></head>
<body>
<h1>llama-swap exporter</h1>
<p><a href="` + *metricsPath + `">Metrics</a></p>
</body>
</html>`))
		if err != nil {
			logger.Error("Error writing response", "err", err)
		}
	})
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	server := &http.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	// Handle graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		logger.Info("Received interrupt signal, shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("HTTP server shutdown error", "err", err)
		}
	}()

	logger.Info("Listening", "address", *listenAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("HTTP server failed", "err", err)
		os.Exit(1)
	}

	logger.Info("llama-swap exporter stopped")
}

type runningModel struct {
	Command     string `json:"cmd"`
	Description string `json:"description"`
	Model       string `json:"model"`
	Name        string `json:"name"`
	Proxy       string `json:"proxy"`
	State       string `json:"state"`
	TTL         int    `json:"ttl"`
}

type runningResponse struct {
	Running []runningModel `json:"running"`
}

type llamaSwapCollector struct {
	ctx           context.Context
	logger        *slog.Logger
	upstreams     []*url.URL
	client        *http.Client
	apiKey        string
	scrapeTimeout time.Duration

	scrapeErrorsTotal *prometheus.CounterVec
	scrapeDuration    *prometheus.HistogramVec
	up                *prometheus.GaugeVec
	modelUp           *prometheus.GaugeVec
	modelInfo         *prometheus.GaugeVec
}

func newLlamaSwapCollector(ctx context.Context, logger *slog.Logger, upstreams []*url.URL, apiKey string, timeout time.Duration) *llamaSwapCollector {
	rt := http.DefaultTransport
	if apiKey != "" {
		rt = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey}),
		}
	}
	return &llamaSwapCollector{
		ctx:           ctx,
		logger:        logger,
		upstreams:     upstreams,
		scrapeTimeout: timeout,
		apiKey:        apiKey,
		client: &http.Client{
			Transport: rt,
			Timeout:   timeout,
		},
		up: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "llama_swap_up",
				Help: "Whether the llama-swap API was up (1) or down (0).",
			},
			[]string{"upstream"},
		),
		scrapeErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "llama_swap_scrape_errors_total",
				Help: "Total number of errors while scraping llama-swap upstreams.",
			},
			[]string{"upstream"},
		),
		scrapeDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "llama_swap_scrape_duration_seconds",
				Help:    "Duration of scrapes from llama-swap upstreams.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"upstream"},
		),
		modelUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "llama_swap_model_up",
				Help: "Whether the llama-swap upstream model target was up (1) or down (0).",
			},
			[]string{"upstream", "model"},
		),
		modelInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "llama_swap_model_info",
				Help: "The discovered llama.cpp models and their current state.",
			},
			[]string{"upstream", "model", "state"},
		),
	}
}

func (c *llamaSwapCollector) Describe(ch chan<- *prometheus.Desc) {
	c.up.Describe(ch)
	c.scrapeErrorsTotal.Describe(ch)
	c.scrapeDuration.Describe(ch)
	c.modelUp.Describe(ch)
	c.modelInfo.Describe(ch)
}

func (c *llamaSwapCollector) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup
	for _, upstream := range c.upstreams {
		wg.Go(func() {
			c.scrapeUpstream(upstream, ch)
		})
	}
	wg.Wait()

	c.up.Collect(ch)
	c.scrapeErrorsTotal.Collect(ch)
	c.scrapeDuration.Collect(ch)
	c.modelUp.Collect(ch)
	c.modelInfo.Collect(ch)
}

func (c *llamaSwapCollector) scrapeUpstream(upstream *url.URL, ch chan<- prometheus.Metric) {
	logger := c.logger.With("upstream", upstream.String())
	start := time.Now()
	models, err := c.fetchRunning(upstream)
	if err != nil {
		logger.Error("failed to query upstream for running models", "err", err)
		c.scrapeErrorsTotal.WithLabelValues(upstream.String()).Inc()
		c.scrapeDuration.WithLabelValues(upstream.String()).Observe(time.Since(start).Seconds())
		c.up.WithLabelValues(upstream.String()).Set(0)
		return
	}
	c.up.WithLabelValues(upstream.String()).Set(1)

	var readyModels []string
	for _, m := range models {
		c.modelInfo.WithLabelValues(upstream.String(), m.Model, m.State).Set(1)
		if m.State == "ready" {
			readyModels = append(readyModels, m.Model)
		}
	}

	if len(readyModels) == 0 {
		c.scrapeDuration.WithLabelValues(upstream.String()).Observe(time.Since(start).Seconds())
		return
	}

	var wg sync.WaitGroup
	for _, model := range readyModels {
		wg.Go(func() {
			logger := logger.With("model", model)
			err := c.scrapeModel(logger, upstream, model, ch)
			if err != nil {
				logger.Error("failed to scrape model", "err", err)
				c.scrapeErrorsTotal.WithLabelValues(upstream.String()).Inc()
				c.modelUp.WithLabelValues(upstream.String(), model).Set(0)
				return
			}
			c.modelUp.WithLabelValues(upstream.String(), model).Set(1)
		})
	}
	wg.Wait()
	c.scrapeDuration.WithLabelValues(upstream.String()).Observe(time.Since(start).Seconds())
}

func (c *llamaSwapCollector) fetchRunning(upstream *url.URL) ([]runningModel, error) {
	req, err := http.NewRequestWithContext(c.ctx, "GET", upstream.JoinPath("running").String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var result runningResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Running, nil
}

func (c *llamaSwapCollector) scrapeModel(logger *slog.Logger, upstream *url.URL, model string, ch chan<- prometheus.Metric) error {
	req, err := http.NewRequestWithContext(c.ctx, "GET", upstream.JoinPath("upstream", model, "metrics").String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/plain; version=0.0.4")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	parser := expfmt.NewTextParser(pmodel.UTF8Validation)
	families, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return err
	}

	for _, family := range families {
		if err := c.colelctMetricFamily(family, upstream, model, ch); err != nil {
			logger.Error("failed to collect metric family", "family", family.GetName(), "err", err)
		}
	}
	return nil
}

func (c *llamaSwapCollector) colelctMetricFamily(family *protomodel.MetricFamily, upstream *url.URL, model string, ch chan<- prometheus.Metric) error {
	name := family.GetName()
	help := family.GetHelp()

	for _, metric := range family.GetMetric() {
		labelNames, labelValues := buildLabels(metric, upstream.String(), model)
		desc := prometheus.NewDesc(name, help, labelNames, nil)

		var pm prometheus.Metric
		var err error

		switch family.GetType() {
		case protomodel.MetricType_GAUGE:
			pm, err = prometheus.NewConstMetric(desc, prometheus.GaugeValue, metric.GetGauge().GetValue(), labelValues...)
		case protomodel.MetricType_COUNTER:
			pm, err = prometheus.NewConstMetric(desc, prometheus.CounterValue, metric.GetCounter().GetValue(), labelValues...)
		case protomodel.MetricType_UNTYPED:
			pm, err = prometheus.NewConstMetric(desc, prometheus.UntypedValue, metric.GetUntyped().GetValue(), labelValues...)
		case protomodel.MetricType_HISTOGRAM:
			h := metric.GetHistogram()
			buckets := make(map[float64]uint64, len(h.GetBucket()))
			for _, b := range h.GetBucket() {
				buckets[b.GetUpperBound()] = b.GetCumulativeCount()
			}
			pm, err = prometheus.NewConstHistogram(desc, h.GetSampleCount(), h.GetSampleSum(), buckets, labelValues...)
		case protomodel.MetricType_SUMMARY:
			s := metric.GetSummary()
			quantiles := make(map[float64]float64, len(s.GetQuantile()))
			for _, q := range s.GetQuantile() {
				quantiles[q.GetQuantile()] = q.GetValue()
			}
			pm, err = prometheus.NewConstSummary(desc, s.GetSampleCount(), s.GetSampleSum(), quantiles, labelValues...)
		default:
			continue
		}

		if err != nil {
			return err
		}
		ch <- pm
	}
	return nil
}

func buildLabels(metric *protomodel.Metric, upstream, model string) ([]string, []string) {
	n := len(metric.Label) + 2
	labelNames := make([]string, 0, n)
	labelValues := make([]string, 0, n)

	for _, l := range metric.Label {
		labelNames = append(labelNames, l.GetName())
		labelValues = append(labelValues, l.GetValue())
	}
	labelNames = append(labelNames, "upstream", "model")
	labelValues = append(labelValues, upstream, model)

	return labelNames, labelValues
}
