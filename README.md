# Order and Payment Microservices

## What matters
- Two independent services with separate Postgres ownership. There is no shared schema, shared table, or shared domain model package.
- `TASK.md` asked for Gin; this implementation intentionally uses `net/http` `ServeMux` plus `go-playground/validator/v10` per the execution requirement.
- Order creation persists `Pending` before the payment call. If payment times out or the Payment Service is unavailable, the API returns `503` and keeps the order `Pending`. That makes retries explicit instead of hiding an uncertain distributed outcome.
- `POST /orders` requires `Idempotency-Key`. The key is bound to a request fingerprint. Same key plus same payload reuses the existing order; same key plus different payload returns `409`.
- Retries against a `Pending` order re-run payment authorization for the same `order_id`. Payment-side uniqueness on `order_id` prevents duplicate transactions if the first request partially succeeded.
- `GET /orders/revenue?customer_id=...` reports realized revenue only. It sums and counts `Paid` orders for that customer; `Pending`, `Failed`, and `Cancelled` orders are excluded.

## Architecture
```mermaid
flowchart LR
    subgraph Client
        C["HTTP client"]
    end

    subgraph OrderService["Order Service"]
        OH["ServeMux handlers"] --> OU["Use cases"]
        OU --> OR["Order repository"]
        OU --> PGW["Payment HTTP client"]
        OR --> ODB[("Order Postgres")]
    end

    subgraph PaymentService["Payment Service"]
        PH["ServeMux handlers"] --> PU["Use cases"]
        PU --> PR["Payment repository"]
        PR --> PDB[("Payment Postgres")]
    end

    C --> OH
    PGW --> PH
```

Mermaid source: [docs/architecture.mmd](/Users/jokeoa/ads2-assignment/docs/architecture.mmd)

## Run
```bash
docker compose up --build
```

Endpoints after startup:
- Order Service: `http://localhost:8080`
- Payment Service: `http://localhost:8081`

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
