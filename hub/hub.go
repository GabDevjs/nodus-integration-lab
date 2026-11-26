// Package hub implements a deterministic, in-memory architectural demo.
// It has no persistence, authentication, networking or concurrent access.
package hub

import (
	"errors"
	"fmt"
)

type Order struct {
	Reference string
	SKU       string
	Quantity  int
}

type PartnerOrder struct {
	ExternalReference string
	ItemCode          string
	Units             int
}

func Map(order Order) PartnerOrder {
	return PartnerOrder{ExternalReference: order.Reference, ItemCode: order.SKU, Units: order.Quantity}
}

type DeliveryStatus string
type CommercialStatus string

const (
	Queued    DeliveryStatus = "QUEUED"
	Delivered DeliveryStatus = "DELIVERED"
	Failed    DeliveryStatus = "FAILED"

	Pending  CommercialStatus = "PENDING"
	Accepted CommercialStatus = "ACCEPTED"
	Rejected CommercialStatus = "REJECTED"
)

type Event struct {
	Kind    string
	Attempt int
}

type Record struct {
	ID         string
	Order      Order
	Delivery   DeliveryStatus
	Commercial CommercialStatus
	Events     []Event
	Attempts   int
}

// Sender models a synthetic partner. It must not be used for real deliveries.
type Sender interface {
	Send(PartnerOrder) error
}

type Hub struct {
	records     map[string]*Record
	keys        map[string]string
	queue       []string
	maxAttempts int
}

func New(maxAttempts int) *Hub {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Hub{records: make(map[string]*Record), keys: make(map[string]string), maxAttempts: maxAttempts}
}

func (h *Hub) Submit(key string, order Order) (string, error) {
	if key == "" || order.Reference == "" || order.SKU == "" || order.Quantity < 1 {
		return "", errors.New("invalid order or key")
	}
	if id, ok := h.keys[key]; ok {
		if h.records[id].Order != order {
			return "", errors.New("idempotency key conflicts with a different order")
		}
		return id, nil
	}
	id := fmt.Sprintf("demo-%04d", len(h.records)+1)
	h.records[id] = &Record{ID: id, Order: order, Delivery: Queued, Commercial: Pending,
		Events: []Event{{Kind: "ORDER_RECEIVED"}}}
	h.keys[key] = id
	h.queue = append(h.queue, id)
	return id, nil
}

// ProcessNext handles one queued event. A failed attempt requeues it only
// within the configured limit. This illustrates sequencing, not a durable outbox.
func (h *Hub) ProcessNext(sender Sender) bool {
	if len(h.queue) == 0 {
		return false
	}
	id := h.queue[0]
	h.queue = h.queue[1:]
	record := h.records[id]
	record.Attempts++
	if err := sender.Send(Map(record.Order)); err != nil {
		if record.Attempts >= h.maxAttempts {
			record.Delivery = Failed
			record.Events = append(record.Events, Event{Kind: "DELIVERY_FAILED", Attempt: record.Attempts})
		} else {
			record.Events = append(record.Events, Event{Kind: "RETRY_QUEUED", Attempt: record.Attempts})
			h.queue = append(h.queue, id)
		}
		return true
	}
	record.Delivery = Delivered
	record.Events = append(record.Events, Event{Kind: "DELIVERED", Attempt: record.Attempts})
	return true
}

// Confirm is separate from delivery. Duplicate identical confirmations
// are harmless, while conflicting decisions are rejected.
func (h *Hub) Confirm(id string, result CommercialStatus) error {
	record, ok := h.records[id]
	if !ok || record.Delivery != Delivered {
		return errors.New("order is not delivered")
	}
	if result != Accepted && result != Rejected {
		return errors.New("invalid commercial result")
	}
	if record.Commercial == result {
		return nil
	}
	if record.Commercial != Pending {
		return errors.New("commercial outcome already recorded")
	}
	record.Commercial = result
	record.Events = append(record.Events, Event{Kind: "COMMERCIAL_" + string(result)})
	return nil
}

func (h *Hub) Record(id string) (Record, bool) {
	record, ok := h.records[id]
	if !ok {
		return Record{}, false
	}
	copyRecord := *record
	copyRecord.Events = append([]Event(nil), record.Events...)
	return copyRecord, true
}
