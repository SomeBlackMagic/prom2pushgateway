# prom2pushgateway

A lightweight Go service that periodically scrapes metrics from an HTTP endpoint and pushes them to a Prometheus Pushgateway.

Supports:
- HTTP(S) targets
- Basic Authentication for Pushgateway
- Custom metric injection
- Config via environment variables only
- Configurable timeouts for both scraping and pushing

---

## 🧱 Environment Variables

| Variable              | Description                                                                                 | Default                                       |
|-----------------------|---------------------------------------------------------------------------------------------|-----------------------------------------------|
| `SOURCE_URL`          | URL of the source metrics endpoint to scrape                                                | `http://app:8080/metrics`                     |
| `PUSHGATEWAY_URL`     | URL of the target Pushgateway endpoint (e.g. `http://pushgateway:9091/metrics/job/example`) | `http://pushgateway:9091/metrics/job/example` |
| `PUSHGATEWAY_USER`    | Optional username for HTTP Basic Auth                                                       | *(empty)*                                     |
| `PUSHGATEWAY_PASS`    | Optional password for HTTP Basic Auth                                                       | *(empty)*                                     |
| `INTERVAL`            | Scrape and push interval in seconds                                                         | `15`                                          |
| `SCRAPE_TIMEOUT`      | Timeout for scraping operation (seconds)                                                    | `5`                                           |
| `PUSH_TIMEOUT`        | Timeout for pushing to Pushgateway (seconds)                                                | `5`                                           |
| `CUSTOM_METRIC_NAME`  | Optional additional metric name to append                                                   | *(empty)*                                     |
| `CUSTOM_METRIC_VALUE` | Optional additional metric value (float)                                                    | `0`                                           |

---

## 🧩 Example: Docker Compose

```yaml
version: "3.8"

services:
  app:
    image: prom/prometheus-example-app
    ports:
      - "8080:8080"

  pushgateway:
    image: prom/pushgateway
    ports:
      - "9091:9091"

  scraper:
    build: .
    environment:
      SOURCE_URL: "http://app:8080/metrics"
      PUSHGATEWAY_URL: "http://pushgateway:9091/metrics/job/example"
      INTERVAL: "10"
      SCRAPE_TIMEOUT: "3"
      PUSH_TIMEOUT: "3"
      CUSTOM_METRIC_NAME: "my_custom_metric"
      CUSTOM_METRIC_VALUE: "42.5"
```

---

## 🔧 Build

```bash
go mod init scraper
go mod tidy
go build -o scraper .
```

or via Docker:

```bash
docker build -t scraper:latest .
```

---

## 🚀 Run

```bash
docker run --rm \
  -e SOURCE_URL=http://app:8080/metrics \
  -e PUSHGATEWAY_URL=http://user:pass@pushgateway:9091/metrics/job/test \
  -e INTERVAL=10 \
  -e SCRAPE_TIMEOUT=3 \
  -e PUSH_TIMEOUT=3 \
  -e CUSTOM_METRIC_NAME=my_custom_value \
  -e CUSTOM_METRIC_VALUE=42.5 \
  scraper:latest
```

---

## 🪶 Logs Example

```
[2025-11-03T15:22:11Z] pushed → http://pushgateway:9091/metrics/job/example (auth=true custom=my_custom_value=42.5)
```

---

## ⚙️ Notes

- Basic Auth credentials can be set via URL or environment variables.
- Both HTTPS and plain HTTP are supported.
- On each cycle:
    1. The app scrapes metrics from `SOURCE_URL`
    2. Optionally adds a custom metric line
    3. Pushes the result to `PUSHGATEWAY_URL`
- Logs are printed to stdout only.

---

## 📜 License

GNU GENERAL PUBLIC LICENSE
