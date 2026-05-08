# llama-swap-exporter

A Prometheus exporter for [llama-swap](https://github.com/mostlygeek/llama-swap) and the underlying [llama.cpp](https://github.com/ggml-org/llama.cpp) model servers, providing metrics for model state, token usage, etc.

[![Build Status](https://github.com/squat/llama-swap-exporter/workflows/CI/badge.svg)](https://github.com/squat/llama-swap-exporter/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/squat/llama-swap-exporter)](https://goreportcard.com/report/github.com/squat/llama-swap-exporter)
[![Built with Nix](https://img.shields.io/static/v1?logo=nixos&logoColor=white&label=&message=Built%20with%20Nix&color=41439a)](https://builtwithnix.org)

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: llama-swap-exporter
    static_configs:
     - targets: [localhost:9293]
    scrape_interval: 30s
    metrics_path: /metrics
```

## Metrics

You can find the full list of metrics in the [METRICS.md](./docs/METRICS.md) file.

## Usage

[embedmd]:# (help.txt)
```txt
Usage of llama-swap-exporter:
  -api-key string
    	Bearer token for llama-swap auth
  -metrics-path string
    	HTTP path on which to serve metrics (default "/metrics")
  -scrape.timeout duration
    	Per-target scrape timeout (default 10s)
  -upstream string
    	Comma-separated llama-swap base URLs
  -version
    	Print the version of llama-swap-exporter and exit
  -web.listen-address string
    	Address on which to serve metrics (default ":9293")
```
