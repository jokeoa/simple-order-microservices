# Order and Payment Microservices

## What matters
- Two independent services with separate Postgres ownership. There is no shared schema, shared table, or shared domain model package.
- `TASK.md` asked for Gin; this implementation intentionally uses `net/http` `ServeMux` plus `go-playground/validator/v10` per the execution requirement.
- Order creation persists `Pending` before the payment call. If the gRPC payment call times out or the Payment Service is unavailable, the API returns `503` and keeps the order `Pending`. That makes retries explicit instead of hiding an uncertain distributed outcome.
- `POST /orders` requires `Idempotency-Key`. The key is bound to a request fingerprint. Same key plus same payload reuses the existing order; same key plus different payload returns `409`.
- Retries against a `Pending` order re-run payment authorization for the same `order_id`. Payment-side uniqueness on `order_id` keeps the gRPC hop safe even when the first attempt partially succeeded.
- `GET /orders/revenue?customer_id=...` reports realized revenue only. It sums and counts `Paid` orders for that customer; `Pending`, `Failed`, and `Cancelled` orders are excluded.
- The `PaymentAuthorizer` seam stayed intact. Only the delivery/infrastructure path changed: Order now calls Payment over gRPC, and Order exposes a separate gRPC stream for internal subscribers.
- `SubscribeToOrderUpdates` is driven by PostgreSQL `LISTEN/NOTIFY` on committed `orders` row changes. The stream is not a timer loop layered on top of fake state.

## Architecture
```mermaid
flowchart LR
    subgraph Client
        C["HTTP client"]
    end

    subgraph OrderService["Order Service"]
        OH["ServeMux handlers"] --> OU["Use cases"]
        OU --> OR["Order repository"]
        OU --> PGC["Payment gRPC client"]
        OGS["Order gRPC stream server"] --> OU
        OR --> ODB[("Order Postgres")]
    end

    subgraph PaymentService["Payment Service"]
        PH["ServeMux handlers"] --> PU["Use cases"]
        PGS["Payment gRPC server"] --> PU
        PU --> PR["Payment repository"]
        PR --> PDB[("Payment Postgres")]
    end

    C --> OH
    PGC --> PGS
    ODB -->|"NOTIFY order_updates"| OGS
```

Mermaid source: [docs/architecture.mmd](/Users/jokeoa/ads2-assignment/docs/architecture.mmd)

## Run
```bash
docker compose up --build
```

Endpoints after startup:
- Order Service: `http://localhost:8080`
- Payment Service: `http://localhost:8081`
- Order gRPC: `localhost:9092`
- Payment gRPC: `localhost:9091`
- Order Frontend: `http://localhost:3000`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3001` (`admin` / `admin`)
- Node Exporter metrics: `http://localhost:9100/metrics`

Required gRPC env vars:
- `PAYMENT_GRPC_ADDR`
- `ORDER_GRPC_ADDR`
- `ORDER_GRPC_STREAM_TIMEOUT`
- `PAYMENT_GRPC_TARGET`
- `PAYMENT_GRPC_TIMEOUT`

The compose file publishes gRPC ports on loopback only (`127.0.0.1`) because they are intended for trusted internal traffic plus local verification, not general external exposure.

## Observability
The repository now includes a default monitoring stack in the `monitoring/` directory:
- `monitoring/prometheus/prometheus.yml` for Prometheus scrape configuration.
- `monitoring/prometheus/rules/slo-rules.yml` for 30-day rolling SLO recording rules.
- `monitoring/grafana/provisioning/datasources/prometheus.yml` for the default Prometheus datasource.
- `monitoring/grafana/provisioning/dashboards/default.yml` for dashboard auto-provisioning.
- `monitoring/grafana/dashboards/node-exporter-overview.json` as a starter infrastructure dashboard.
- `monitoring/grafana/dashboards/sre-golden-signals.json` for application golden signals and SLO compliance.

Start everything with:
```bash
docker compose up --build
```

Prometheus currently scrapes:
- `prometheus:9090`
- `order-service:8080/metrics`
- `payment-service:8081/metrics`
- `node-exporter:9100`

Application metrics now include:
- Inbound HTTP traffic, latency, and error metrics for both services.
- Internal order-service to payment-service authorization latency and error metrics.
- Saturation signals from Go runtime goroutines and Postgres connection pool utilization.
- Two SLO compliance recording rules:
  - Order creation availability: `99.5%` over a 30-day rolling window.
  - Payment authorization latency under `1500 ms`: `95%` over a 30-day rolling window.

## API examples
Create an order:
```bash
curl -i http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: order-001' \
  -d '{"customer_id":"cust-1","item_name":"book","amount":500}'
```

Replay the same logical request safely:
```bash
curl -i http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: order-001' \
  -d '{"customer_id":"cust-1","item_name":"book","amount":500}'
```

Payload mismatch on the same idempotency key:
```bash
curl -i http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: order-001' \
  -d '{"customer_id":"cust-1","item_name":"pen","amount":500}'
```

Get or cancel an order:
```bash
curl -i http://localhost:8080/orders/<order-id>
curl -i -X PATCH http://localhost:8080/orders/<order-id>/cancel
```

Get paid-order revenue for a customer:
```bash
curl -i "http://localhost:8080/orders/revenue?customer_id=c1"
```

Get a payment decision directly:
```bash
curl -i http://localhost:8081/payments/<order-id>
```

Generate protobuf stubs:
```bash
make proto-tools
make proto
```

Watch order updates over gRPC:
```bash
ORDER_GRPC_TARGET=localhost:9092 go run ./cmd/order-stream-client <order-id>
```

End-to-end streaming check:
1. Start the stack with `docker compose up --build`.
2. Create an order with `POST /orders`.
3. In a second terminal, run `ORDER_GRPC_TARGET=localhost:9092 go run ./cmd/order-stream-client <order-id>`.
4. Retry the same order or cancel a pending one to observe a real database-backed status update on the stream.

## Testing
Unit and HTTP tests:
```bash
go test ./...
```

Repository integration tests run when DSNs are provided:
```bash
TEST_ORDER_POSTGRES_DSN='postgres://order:order@localhost:5433/orders?sslmode=disable' \
TEST_PAYMENT_POSTGRES_DSN='postgres://payment:payment@localhost:5434/payments?sslmode=disable' \
  go test ./...
```
