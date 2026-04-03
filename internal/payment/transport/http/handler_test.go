package http

import (
	"context"
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
}

func (s *paymentServiceStub) Authorize(ctx context.Context, input usecase.AuthorizeInput) (domain.Payment, bool, error) {
	return s.authorizeFunc(ctx, input)
}

func (s *paymentServiceStub) GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error) {
	return s.getFunc(ctx, orderID)
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
