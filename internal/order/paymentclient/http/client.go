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

	"github.com/jokeoa/simple-order-microservices/internal/order/usecase"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
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

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
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

	response, err := c.httpClient.Do(request)
	if err != nil {
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
		return usecase.PaymentResult{}, usecase.ErrPaymentUnavailable
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		var errorPayload errorResponse
		if err := json.NewDecoder(response.Body).Decode(&errorPayload); err == nil && errorPayload.Error != "" {
			return usecase.PaymentResult{}, fmt.Errorf("payment service rejected request: %s", errorPayload.Error)
		}

		return usecase.PaymentResult{}, fmt.Errorf("payment service returned status %d", response.StatusCode)
	}

	var payloadResponse createPaymentResponse
	if err := json.NewDecoder(response.Body).Decode(&payloadResponse); err != nil {
		return usecase.PaymentResult{}, fmt.Errorf("decode payment response: %w", err)
	}

	return usecase.PaymentResult{
		Status:        payloadResponse.Status,
		TransactionID: payloadResponse.TransactionID,
	}, nil
}
