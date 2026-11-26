package hub

import (
	"errors"
	"testing"
)

type stub struct {
	failures int
	calls    []PartnerOrder
}

func (s *stub) Send(order PartnerOrder) error {
	s.calls = append(s.calls, order)
	if len(s.calls) <= s.failures {
		return errors.New("synthetic failure")
	}
	return nil
}

func TestIntakeDeduplication(t *testing.T) {
	h := New(2)
	order := Order{Reference: "demo", SKU: "widget", Quantity: 2}
	first, err := h.Submit("key", order)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.Submit("key", order)
	if err != nil || first != second {
		t.Fatalf("deduplication failed: %s %s %v", first, second, err)
	}
	changed := order
	changed.Quantity = 3
	if _, err := h.Submit("key", changed); err == nil {
		t.Fatal("expected conflict")
	}
	sender := &stub{}
	if !h.ProcessNext(sender) || h.ProcessNext(sender) || len(sender.calls) != 1 {
		t.Fatal("duplicate order enqueued")
	}
	if got := sender.calls[0]; got != Map(order) {
		t.Fatalf("unexpected mapping: %+v", got)
	}
}

func TestBoundedRetryAndCommercialDecision(t *testing.T) {
	h := New(2)
	id, err := h.Submit("test", Order{Reference: "synthetic", SKU: "widget", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Confirm(id, Accepted); err == nil {
		t.Fatal("accepted before technical delivery")
	}
	s := &stub{failures: 1}
	h.ProcessNext(s)
	record, _ := h.Record(id)
	if record.Delivery != Queued || record.Commercial != Pending || record.Attempts != 1 {
		t.Fatalf("unexpected retry state: %+v", record)
	}
	h.ProcessNext(s)
	record, _ = h.Record(id)
	if record.Delivery != Delivered || record.Commercial != Pending || record.Attempts != 2 {
		t.Fatalf("unexpected delivered state: %+v", record)
	}
	if err := h.Confirm(id, Accepted); err != nil {
		t.Fatal(err)
	}
	if err := h.Confirm(id, Accepted); err != nil {
		t.Fatal("repeated decision must be harmless:", err)
	}
	if err := h.Confirm(id, Rejected); err == nil {
		t.Fatal("conflicting decision should be rejected")
	}
	record, _ = h.Record(id)
	if record.Commercial != Accepted || len(record.Events) != 4 {
		t.Fatalf("unexpected outcome/timeline: %+v", record)
	}
}

func TestAttemptsExhausted(t *testing.T) {
	h := New(2)
	id, err := h.Submit("test", Order{Reference: "synthetic", SKU: "widget", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	s := &stub{failures: 99}
	h.ProcessNext(s)
	h.ProcessNext(s)
	if h.ProcessNext(s) {
		t.Fatal("retry budget exceeded")
	}
	record, _ := h.Record(id)
	if record.Delivery != Failed || record.Commercial != Pending || record.Attempts != 2 {
		t.Fatalf("unexpected terminal state: %+v", record)
	}
}

func TestRecordIsReadOnlySnapshot(t *testing.T) {
	h := New(1)
	id, _ := h.Submit("test", Order{Reference: "synthetic", SKU: "widget", Quantity: 1})
	record, _ := h.Record(id)
	record.Events[0].Kind = "mutated"
	current, _ := h.Record(id)
	if current.Events[0].Kind != "ORDER_RECEIVED" {
		t.Fatal("record leaked mutable timeline")
	}
}
