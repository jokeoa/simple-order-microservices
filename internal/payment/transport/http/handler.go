package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jokeoa/simple-order-microservices/internal/payment/domain"
	"github.com/jokeoa/simple-order-microservices/internal/payment/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/httpx"
	"github.com/jokeoa/simple-order-microservices/internal/platform/validate"
)

type paymentService interface {
	Authorize(ctx context.Context, input usecase.AuthorizeInput) (domain.Payment, bool, error)
	GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error)
	ListPayments(ctx context.Context, min, max int64) ([]domain.Payment, error)
}

type Handler struct {
	service   paymentService
	validator *validate.Validator
}

type createPaymentRequest struct {
	OrderID       string `json:"order_id" validate:"required"`
	Amount        int64  `json:"amount" validate:"gt=0"`
	CustomerEmail string `json:"customer_email"`
}

type paymentResponse struct {
	OrderID       string        `json:"order_id"`
	Amount        int64         `json:"amount"`
	Status        domain.Status `json:"status"`
	TransactionID string        `json:"transaction_id,omitempty"`
}

type listPaymentsResponse struct {
	Payments []paymentResponse `json:"payments"`
}

func NewHandler(service paymentService, validator *validate.Validator) *Handler {
	return &Handler{service: service, validator: validator}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /payments", h.listPayments)
	mux.HandleFunc("POST /payments", h.createPayment)
	mux.HandleFunc("GET /payments/{orderID}", h.getPayment)
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	minAmount, err := parseAmountQueryParam(r, "min_amount")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	maxAmount, err := parseAmountQueryParam(r, "max_amount")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	payments, err := h.service.ListPayments(r.Context(), minAmount, maxAmount)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidAmountRange) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid amount range")
			return
		}

		httpx.WriteError(w, http.StatusInternalServerError, "failed to list payments")
		return
	}

	response := listPaymentsResponse{
		Payments: make([]paymentResponse, 0, len(payments)),
	}
	for _, payment := range payments {
		response.Payments = append(response.Payments, mapPayment(payment))
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) createPayment(w http.ResponseWriter, r *http.Request) {
	var request createPaymentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validator.Struct(request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	payment, created, err := h.service.Authorize(r.Context(), usecase.AuthorizeInput{
		OrderID:       request.OrderID,
		Amount:        request.Amount,
		CustomerEmail: request.CustomerEmail,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to authorize payment")
		return
	}

	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}

	httpx.WriteJSON(w, status, mapPayment(payment))
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	payment, err := h.service.GetByOrderID(r.Context(), r.PathValue("orderID"))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "payment not found")
			return
		}

		httpx.WriteError(w, http.StatusInternalServerError, "failed to load payment")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, mapPayment(payment))
}

func mapPayment(payment domain.Payment) paymentResponse {
	response := paymentResponse{
		OrderID: payment.OrderID,
		Amount:  payment.Amount,
		Status:  payment.Status,
	}
	if payment.TransactionID != "" {
		response.TransactionID = payment.TransactionID
	}

	return response
}

func parseAmountQueryParam(r *http.Request, key string) (int64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}

	amount, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.New(key + " must be a valid int64")
	}

	return amount, nil
}
