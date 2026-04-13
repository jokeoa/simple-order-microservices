package stream

import (
	"slices"
	"sync"
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/order/domain"
)

type Update struct {
	OrderID              string
	Status               domain.Status
	PaymentTransactionID string
	Timestamp            time.Time
}

type Broker struct {
	mu          sync.RWMutex
	nextID      uint64
	closed      bool
	subscribers map[string]map[uint64]chan Update
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string]map[uint64]chan Update),
	}
}

func (b *Broker) Subscribe(orderID string) (<-chan Update, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	updates := make(chan Update, 1)
	if b.closed {
		close(updates)
		return updates, func() {}
	}

	b.nextID++
	subscriberID := b.nextID
	if b.subscribers[orderID] == nil {
		b.subscribers[orderID] = make(map[uint64]chan Update)
	}
	b.subscribers[orderID][subscriberID] = updates

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if subscribers := b.subscribers[orderID]; subscribers != nil {
				if channel, ok := subscribers[subscriberID]; ok {
					delete(subscribers, subscriberID)
					close(channel)
				}
				if len(subscribers) == 0 {
					delete(b.subscribers, orderID)
				}
			}
		})
	}

	return updates, cancel
}

func (b *Broker) Publish(update Update) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, subscriber := range b.subscribers[update.OrderID] {
		sendLatest(subscriber, update)
	}
}

func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	for orderID, subscribers := range b.subscribers {
		for subscriberID, channel := range subscribers {
			close(channel)
			delete(subscribers, subscriberID)
		}
		delete(b.subscribers, orderID)
	}
}

func (b *Broker) OrderIDs() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	orderIDs := make([]string, 0, len(b.subscribers))
	for orderID, subscribers := range b.subscribers {
		if len(subscribers) == 0 {
			continue
		}
		orderIDs = append(orderIDs, orderID)
	}
	slices.Sort(orderIDs)

	return orderIDs
}

func sendLatest(channel chan Update, update Update) {
	select {
	case channel <- update:
	default:
		select {
		case <-channel:
		default:
		}
		channel <- update
	}
}
