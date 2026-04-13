package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
	"github.com/jokeoa/simple-order-microservices/internal/platform/metrics"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	metrics    *metrics.PaymentClientMetrics
}

type createPaymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

type createPaymentResponse struct {
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(baseURL string, httpClient *http.Client, clientMetrics *metrics.PaymentClientMetrics) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		metrics:    clientMetrics,
	}
}

func (c *Client) Authorize(ctx context.Context, input usecase.PaymentInput) (usecase.PaymentResult, error) {
	endpoint, err := url.JoinPath(c.baseURL, "/payments")
	if err != nil {
		return usecase.PaymentResult{}, fmt.Errorf("build payment endpoint: %w", err)
	}

	payload, err := json.Marshal(createPaymentRequest{OrderID: input.OrderID, Amount: input.Amount})
	if err != nil {
		return usecase.PaymentResult{}, fmt.Errorf("marshal payment request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return usecase.PaymentResult{}, fmt.Errorf("build payment request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	startedAt := time.Now()
	response, err := c.httpClient.Do(request)
	if err != nil {
		c.observe(http.MethodPost, "/payments", 0, classifyTransportOutcome(err), time.Since(startedAt))

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return usecase.PaymentResult{}, usecase.ErrPaymentUnavailable
		}

		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			if errors.Is(urlErr.Err, context.DeadlineExceeded) {
				return usecase.PaymentResult{}, usecase.ErrPaymentUnavailable
			}
		}

		return usecase.PaymentResult{}, usecase.ErrPaymentUnavailable
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusInternalServerError {
		c.observe(http.MethodPost, "/payments", response.StatusCode, "server_error", time.Since(startedAt))
		return usecase.PaymentResult{}, usecase.ErrPaymentUnavailable
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		c.observe(http.MethodPost, "/payments", response.StatusCode, "client_error", time.Since(startedAt))

		var errorPayload errorResponse
		if err := json.NewDecoder(response.Body).Decode(&errorPayload); err == nil && errorPayload.Error != "" {
			return usecase.PaymentResult{}, fmt.Errorf("payment service rejected request: %s", errorPayload.Error)
		}

		return usecase.PaymentResult{}, fmt.Errorf("payment service returned status %d", response.StatusCode)
	}

	var payloadResponse createPaymentResponse
	if err := json.NewDecoder(response.Body).Decode(&payloadResponse); err != nil {
		c.observe(http.MethodPost, "/payments", response.StatusCode, "invalid_response", time.Since(startedAt))
		return usecase.PaymentResult{}, fmt.Errorf("decode payment response: %w", err)
	}

	c.observe(http.MethodPost, "/payments", response.StatusCode, "success", time.Since(startedAt))

	return usecase.PaymentResult{
		Status:        payloadResponse.Status,
		TransactionID: payloadResponse.TransactionID,
	}, nil
}

func (c *Client) observe(method, route string, statusCode int, outcome string, duration time.Duration) {
	if c.metrics == nil {
		return
	}

	c.metrics.Observe(method, route, statusCode, outcome, duration)
}

func classifyTransportOutcome(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		switch {
		case errors.Is(urlErr.Err, context.DeadlineExceeded):
			return "timeout"
		case errors.Is(urlErr.Err, context.Canceled):
			return "canceled"
		}
	}

	return "transport_error"
}
