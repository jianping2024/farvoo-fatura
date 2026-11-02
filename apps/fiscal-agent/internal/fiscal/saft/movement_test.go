package saft

import (
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// StockMovement ShipTo/ShipFrom follow the SAF-T PT 1.04_01 XSD: ShippingPointStructure with
// a full AddressStructure, placed before MovementStartTime (not after DocumentTotals).
func TestWriteStockMovement_ShippingPoints(t *testing.T) {
	var b strings.Builder
	writeStockMovement(&b, store.SAFTStockMovement{
		DocumentNumber: "GR GR2026T02/1", MovementType: "GR", MovementStartAt: "2026-11-02T15:00:00",
		ShipFrom: domain.Address{Detail: "Rua A 1", City: "Lisboa", PostalCode: "1000-001"},
		ShipTo:   domain.Address{Detail: "Legacy free text"}, // pre-017 row
	}, func(string, string) {})
	x := b.String()
	shipTo := `<ShipTo><Address><AddressDetail>Legacy free text</AddressDetail><City>Desconhecido</City><PostalCode>Desconhecido</PostalCode><Country>PT</Country></Address></ShipTo>`
	shipFrom := `<ShipFrom><Address><AddressDetail>Rua A 1</AddressDetail><City>Lisboa</City><PostalCode>1000-001</PostalCode><Country>PT</Country></Address></ShipFrom>`
	order := []string{"<SourceID>", shipTo, shipFrom, "<MovementStartTime>", "<DocumentTotals>"}
	last := -1
	for _, part := range order {
		i := strings.Index(x, part)
		if i < 0 || i < last {
			t.Fatalf("%q missing or out of XSD order in\n%s", part, x)
		}
		last = i
	}
}
