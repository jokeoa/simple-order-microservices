package events

import "time"

const (
	PaymentCompletedSubject = "payment.completed"
	PaymentCompletedDLQ     = "payment.completed.dlq"
)

type PaymentCompleted struct {
	MessageID     string    `json:"message_id"`
	OrderID       string    `json:"order_id"`
	Amount        int64     `json:"amount"`
	CustomerEmail string    `json:"customer_email"`
	Status        string    `json:"status"`
	OccurredAt    time.Time `json:"occurred_at"`
}
