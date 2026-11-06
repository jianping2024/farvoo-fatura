package saft

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestSAFTCustomerIdentity(t *testing.T) {
	nif := saftCustomer(store.SAFTCustomer{CustomerTaxID: "326588639", CompanyName: "Cliente Teste Lda"})
	if nif.ID != "326588639" || nif.CustomerTaxID != "326588639" {
		t.Fatalf("NIF customer: %+v", nif)
	}
	for _, c := range []store.SAFTCustomer{
		{CompanyName: "Consumidor Final", AddressDetail: "Avenida da Liberdade", City: "Lisboa"},
		{CustomerTaxID: "999999990", CompanyName: "consumidor final"},
		{},
	} {
		if got := saftCustomer(c); got.ID != "CF" || got.CustomerTaxID != "999999990" || got.AddressDetail != "Desconhecido" || got.CompanyName != "Consumidor Final" {
			t.Fatalf("final consumer %+v → %+v", c, got)
		}
	}
	maria := store.SAFTCustomer{CompanyName: "Maria Costa", AddressDetail: "Rua 3", City: "Porto", PostalCode: "4000-002"}
	joao := maria
	joao.CompanyName = "João Silva"
	m1, m2, j := saftCustomer(maria), saftCustomer(maria), saftCustomer(joao)
	if m1.ID != m2.ID || m1.ID == j.ID || m1.ID == "CF" || len(m1.ID) > 30 || m1.CustomerTaxID != "999999990" {
		t.Fatalf("named without NIF must be stable, distinct, ≤30: %q %q %q", m1.ID, m2.ID, j.ID)
	}
}
