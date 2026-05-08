# llama-swap-exporter Metrics

This document describes all the Prometheus metrics exported by the exporter.

## llama-swap Metrics

### General Metrics

These are core metrics provided by the exporter concerning the collection of metrics:

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `llama_swap_up` | Gauge | Whether llama-swap API is accessible | `upstream` |
| `llama_swap_scrape_duration_seconds` | Histogram | Duration of a collector scrape | `upstream` |
| `llama_swap_scrape_errors_total` | Counter | Total number of errors while scraping llama-swap upstreams | `upstream` |

### Model Metrics

These metrics concern models running in llama-swap:

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `llama_swap_model_info` | Gauge | Model information | `model`, `state`, `upstream` |
| `llama_swap_model_up` | Gauge | Whether the model API is accessible | `model`, `upstream` |

## llama.cpp Metrics

### General Metrics

These are metrics exposed by the underlying llama.cpp model servers:

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `llamacpp:n_busy_slots_per_decode` | Counter | Average number of busy slots per llama_decode() call | `model`, `upstream` |
| `llamacpp:n_decode_total` | Counter | Total number of llama_decode() calls | `model`, `upstream` |
| `llamacpp:predicted_tokens_seconds` | Gauge | Average generation throughput in tokens/s | `model`, `upstream` |
| `llamacpp:prompt_seconds_total` | Counter | Prompt process time | `model`, `upstream` |
| `llamacpp:prompt_tokens_seconds` | Gauge | Average prompt throughput in tokens/s | `model`, `upstream` |
| `llamacpp:prompt_tokens_total` | Counter | Number of prompt tokens processed | `model`, `upstream` |
| `llamacpp:requests_deferred` | Gauge | Number of requests deferred | `model`, `upstream` |
| `llamacpp:requests_processing` | Gauge | Number of requests processing | `model`, `upstream` |
| `llamacpp:tokens_predicted_seconds_total` | Counter | Predict process time | `model`, `upstream` |
| `llamacpp:tokens_predicted_total` | Counter | Number of generation tokens processed | `model`, `upstream` |
