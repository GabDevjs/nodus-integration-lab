package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/GabDevjs/nodus-integration-lab/hub"
)

type simulatedPartner struct{ attempts int }

func (p *simulatedPartner) Send(order hub.PartnerOrder) error {
	p.attempts++
	if p.attempts == 1 {
		return errors.New("synthetic transient failure")
	}
	fmt.Printf("Simulated delivery of %s (%d units)\n", order.ItemCode, order.Units)
	return nil
}

func main() {
	h := hub.New(2)
	input := hub.Order{Reference: "SYNTHETIC-01", SKU: "WIDGET", Quantity: 3}
	id, err := h.Submit("demo-key", input)
	if err != nil {
		log.Fatal(err)
	}
	second, err := h.Submit("demo-key", input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Intake deduplicated: %t\n", id == second)

	partner := &simulatedPartner{}
	for h.ProcessNext(partner) {
	}
	if err := h.Confirm(id, hub.Accepted); err != nil {
		log.Fatal(err)
	}
	record, _ := h.Record(id)
	for _, event := range record.Events {
		fmt.Printf("%s (attempt %d)\n", event.Kind, event.Attempt)
	}
	fmt.Printf("Delivery: %s | Commercial: %s\n", record.Delivery, record.Commercial)
}
