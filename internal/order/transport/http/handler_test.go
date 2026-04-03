package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/validate"
)

type orderServiceStub struct {
	createFunc func(context.Context, usecase.CreateOrderInput) (domain.Order, bool, error)
}

func (s *orderServiceStub) Create(ctx context.Context, input usecase.CreateOrderInput) (domain.Order, bool, error) {
	return s.createFunc(ctx, input)
}

func (s *orderServiceStub) GetByID(context.Context, string) (domain.Order, error) {
	return domain.Order{}, nil
}

func (s *orderServiceStub) Cancel(context.Context, string) (domain.Order, error) {
	return domain.Order{}, nil
}

func TestCreateOrderRequiresIdempotencyKey(t *testing.T) {
	handler := NewHandler(&orderServiceStub{createFunc: func(context.Context, usecase.CreateOrderInput) (domain.Order, bool, error) {
		return domain.Order{}, true, nil
	}}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"customer_id":"cust-1","item_name":"book","amount":100}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCreateOrderReturnsConflictOnPayloadMismatch(t *testing.T) {
	handler := NewHandler(&orderServiceStub{createFunc: func(context.Context, usecase.CreateOrderInput) (domain.Order, bool, error) {
		return domain.Order{}, false, usecase.ErrIdempotencyConflict
	}}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"customer_id":"cust-1","item_name":"book","amount":100}`))
	request.Header.Set("Idempotency-Key", "key-1")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestCreateOrderReturnsServiceUnavailableWithPendingOrder(t *testing.T) {
	pending := domain.Order{ID: "order-1", Status: domain.StatusPending}
	handler := NewHandler(&orderServiceStub{createFunc: func(context.Context, usecase.CreateOrderInput) (domain.Order, bool, error) {
		return pending, true, usecase.ErrPaymentUnavailable
	}}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"customer_id":"cust-1","item_name":"book","amount":100}`))
	request.Header.Set("Idempotency-Key", "key-1")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != string(domain.StatusPending) {
		t.Fatalf("body status = %v, want %s", body["status"], domain.StatusPending)
	}
}
