package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/validate"
)

type paymentServiceStub struct {
	authorizeFunc func(context.Context, usecase.AuthorizeInput) (domain.Payment, bool, error)
	getFunc       func(context.Context, string) (domain.Payment, error)
	listFunc      func(context.Context, int64, int64) ([]domain.Payment, error)
}

func (s *paymentServiceStub) Authorize(ctx context.Context, input usecase.AuthorizeInput) (domain.Payment, bool, error) {
	return s.authorizeFunc(ctx, input)
}

func (s *paymentServiceStub) GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error) {
	return s.getFunc(ctx, orderID)
}

func (s *paymentServiceStub) ListPayments(ctx context.Context, min, max int64) ([]domain.Payment, error) {
	return s.listFunc(ctx, min, max)
}

func TestCreatePaymentReturnsOKWhenReplayed(t *testing.T) {
	handler := NewHandler(&paymentServiceStub{authorizeFunc: func(context.Context, usecase.AuthorizeInput) (domain.Payment, bool, error) {
		return domain.Payment{OrderID: "order-1", Status: domain.StatusAuthorized, TransactionID: "tx-1"}, false, nil
	}}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"order_id":"order-1","amount":100}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestListPaymentsReturnsFilteredPayments(t *testing.T) {
	handler := NewHandler(&paymentServiceStub{
		authorizeFunc: func(context.Context, usecase.AuthorizeInput) (domain.Payment, bool, error) {
			return domain.Payment{}, false, nil
		},
		getFunc: func(context.Context, string) (domain.Payment, error) {
			return domain.Payment{}, nil
		},
		listFunc: func(_ context.Context, min, max int64) ([]domain.Payment, error) {
			if min != 100 || max != 200 {
				t.Fatalf("ListPayments called with [%d,%d], want [100,200]", min, max)
			}
			return []domain.Payment{
				{OrderID: "order-2", Amount: 200, Status: domain.StatusAuthorized, TransactionID: "tx-2"},
				{OrderID: "order-1", Amount: 100, Status: domain.StatusDeclined},
			}, nil
		},
	}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/payments?min_amount=100&max_amount=200", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var payload listPaymentsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Payments) != 2 {
		t.Fatalf("len(payments) = %d, want 2", len(payload.Payments))
	}
	if payload.Payments[0].OrderID != "order-2" {
		t.Fatalf("first order_id = %q, want order-2", payload.Payments[0].OrderID)
	}
}

func TestListPaymentsRejectsInvalidQueryParam(t *testing.T) {
	handler := NewHandler(&paymentServiceStub{
		authorizeFunc: func(context.Context, usecase.AuthorizeInput) (domain.Payment, bool, error) {
			return domain.Payment{}, false, nil
		},
		getFunc: func(context.Context, string) (domain.Payment, error) {
			return domain.Payment{}, nil
		},
		listFunc: func(context.Context, int64, int64) ([]domain.Payment, error) {
			t.Fatal("ListPayments should not be called")
			return nil, nil
		},
	}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/payments?min_amount=abc", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestListPaymentsRejectsInvalidRange(t *testing.T) {
	handler := NewHandler(&paymentServiceStub{
		authorizeFunc: func(context.Context, usecase.AuthorizeInput) (domain.Payment, bool, error) {
			return domain.Payment{}, false, nil
		},
		getFunc: func(context.Context, string) (domain.Payment, error) {
			return domain.Payment{}, nil
		},
		listFunc: func(context.Context, int64, int64) ([]domain.Payment, error) {
			return nil, usecase.ErrInvalidAmountRange
		},
	}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/payments?min_amount=200&max_amount=100", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestListPaymentsReturnsInternalError(t *testing.T) {
	handler := NewHandler(&paymentServiceStub{
		authorizeFunc: func(context.Context, usecase.AuthorizeInput) (domain.Payment, bool, error) {
			return domain.Payment{}, false, nil
		},
		getFunc: func(context.Context, string) (domain.Payment, error) {
			return domain.Payment{}, nil
		},
		listFunc: func(context.Context, int64, int64) ([]domain.Payment, error) {
			return nil, errors.New("boom")
		},
	}, validate.New())
	mux := http.NewServeMux()
	handler.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/payments", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}
