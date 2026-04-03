package domain

import "time"

type Status string

const (
	StatusPending   Status = "Pending"
	StatusPaid      Status = "Paid"
	StatusFailed    Status = "Failed"
	StatusCancelled Status = "Cancelled"
)

type Order struct {
	ID                   string
	CustomerID           string
	ItemName             string
	Amount               int64
	Status               Status
	IdempotencyKey       string
	RequestFingerprint   string
	PaymentTransactionID string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
