package domain

import "time"

type Status string

const (
	StatusAuthorized Status = "Authorized"
	StatusDeclined   Status = "Declined"
)

type Payment struct {
	OrderID       string
	Amount        int64
	Status        Status
	TransactionID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
