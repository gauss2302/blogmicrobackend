# Monitoring (Prometheus + Grafana)

The stack includes **Prometheus** for metrics collection and **Grafana** for dashboards. Both are attached to `internal_net` and are started with **`make infra-up`** (alongside Redis, Postgres, RabbitMQ, OpenSearch, Kafka).

## URLs (defaults)

| Tool        | URL                         | Notes                                      |
|------------|-----------------------------|--------------------------------------------|
| Grafana    | http://localhost:3001       | Override with `GRAFANA_PORT`             |
| Prometheus | http://localhost:9090       | Override with `PROMETHEUS_PORT`          |
| API Gateway| http://localhost:8080     | Only public app entry; `/metrics` there |

Log in to Grafana with `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` (see `.env.example`).

## Prometheus targets

Open **Prometheus → Status → Targets**. All jobs should be **UP** once **`make app-up`** (or full stack) is running:

- `api-gateway:8080`
- `auth-service:8081`
- `user-service:8082`
- `post-service:8083`
- `notification-service:8084`
- `search-service:9095` (dedicated metrics HTTP; gRPC remains on `50054`)

If you change service HTTP ports in Compose, update **`monitoring/prometheus/prometheus.yml`** `static_configs.targets` to match. For search, the metrics port is **`SEARCH_SERVICE_METRICS_HTTP_PORT`** (default `9095`); keep the scrape target in sync if you override it.

## Application metrics

Custom metrics use the `microblog` namespace:

- **HTTP**: `microblog_http_requests_total`, `microblog_http_request_duration_seconds` (labels: `service`, `method`, `route`, `status` where applicable).
- **gRPC** (where the service exposes a gRPC server): `microblog_grpc_server_requests_total`, `microblog_grpc_server_request_duration_seconds`.

Go runtime metrics from the default collectors: e.g. `go_goroutines`, `go_memstats_*`.

## Sample PromQL

```promql
up
```

```promql
sum by (service) (rate(microblog_http_requests_total[5m]))
```

```promql
sum by (service) (rate(microblog_grpc_server_requests_total[5m]))
```

```promql
histogram_quantile(0.95, sum by (le, service) (rate(microblog_grpc_server_request_duration_seconds_bucket[5m])))
```

## Grafana dashboards

Provisioning loads:

- Datasource: Prometheus (`http://prometheus:9090`)
- Dashboard: **Microblog overview** (`monitoring/grafana/dashboards/microblog-overview.json`)

After first start, open **Dashboards** and select **Microblog overview**.

## Local hybrid run

When running a service with `go run` outside Docker, Prometheus inside Compose cannot reach `localhost` on the host. Either run the full stack in Compose or add a **host.docker.internal** scrape job for development (not committed by default).
