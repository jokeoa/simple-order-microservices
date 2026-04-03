package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/httpx"
	"github.com/jokeoa/simple-order-microservices/internal/platform/validate"
)

type orderService interface {
	Create(ctx context.Context, input usecase.CreateOrderInput) (domain.Order, bool, error)
	GetByID(ctx context.Context, id string) (domain.Order, error)
	Cancel(ctx context.Context, id string) (domain.Order, error)
}

type Handler struct {
	service   orderService
	validator *validate.Validator
}

type createOrderRequest struct {
	CustomerID string `json:"customer_id" validate:"required"`
	ItemName   string `json:"item_name" validate:"required"`
	Amount     int64  `json:"amount" validate:"gt=0"`
}

type orderResponse struct {
	ID                   string        `json:"id"`
	CustomerID           string        `json:"customer_id"`
	ItemName             string        `json:"item_name"`
	Amount               int64         `json:"amount"`
	Status               domain.Status `json:"status"`
	PaymentTransactionID string        `json:"payment_transaction_id,omitempty"`
}

func NewHandler(service orderService, validator *validate.Validator) *Handler {
	return &Handler{service: service, validator: validator}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.createOrder)
	mux.HandleFunc("GET /orders/{id}", h.getOrder)
	mux.HandleFunc("PATCH /orders/{id}/cancel", h.cancelOrder)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	var request createOrderRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validator.Struct(request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	order, created, err := h.service.Create(r.Context(), usecase.CreateOrderInput{
		IdempotencyKey: idempotencyKey,
		CustomerID:     request.CustomerID,
		ItemName:       request.ItemName,
		Amount:         request.Amount,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrPaymentUnavailable):
			httpx.WriteJSON(w, http.StatusServiceUnavailable, mapOrder(order))
		case errors.Is(err, usecase.ErrIdempotencyConflict):
			httpx.WriteError(w, http.StatusConflict, "idempotency key payload mismatch")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to create order")
		}
		return
	}

	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}

	httpx.WriteJSON(w, status, mapOrder(order))
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "order not found")
			return
		}

		httpx.WriteError(w, http.StatusInternalServerError, "failed to load order")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, mapOrder(order))
}

func (h *Handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.Cancel(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, usecase.ErrConflict):
			httpx.WriteError(w, http.StatusConflict, "order cannot be cancelled")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to cancel order")
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, mapOrder(order))
}

func mapOrder(order domain.Order) orderResponse {
	response := orderResponse{
		ID:         order.ID,
		CustomerID: order.CustomerID,
		ItemName:   order.ItemName,
		Amount:     order.Amount,
		Status:     order.Status,
	}
	if order.PaymentTransactionID != "" {
		response.PaymentTransactionID = order.PaymentTransactionID
	}

	return response
}
