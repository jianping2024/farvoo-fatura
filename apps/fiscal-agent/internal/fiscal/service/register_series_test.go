package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestRegisterSeries_IdempotentAndRejectSecondCode(t *testing.T) {
	t.Setenv("FISCAL_AT_ENV", "mock")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-demo-001"
	if err := db.UpsertTaxpayer(store.TaxpayerInput{
		StoreID: storeID, TaxRegistrationNumber: "517535009", LegalName: "Demo",
		AddressDetail: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}

	svc := service.New(db, nil, nil, filepath.Join(dir, "secure"), storeID)
	if err := svc.UpsertATCredentials(storeID, "517535009/37", "demo-secret"); err != nil {
		t.Fatal(err)
	}

	year := 2026
	r1, err := svc.RegisterSeries(context.Background(), storeID, "FS2026ONLY01", "FS", year)
	if err != nil {
		t.Fatal(err)
	}
	if r1.IdempotentHit || r1.SeriesCode != "FS2026ONLY01" {
		t.Fatalf("first register %+v", r1)
	}

	r2, err := svc.RegisterSeries(context.Background(), storeID, "FS2026ONLY01", "FS", year)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.IdempotentHit {
		t.Fatal("same code should be idempotent")
	}

	_, err = svc.RegisterSeries(context.Background(), storeID, "FS2026OTHER02", "FS", year)
	if err == nil {
		t.Fatal("expected series_already_active")
	}
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != service.ErrCodeSeriesAlreadyActive {
		t.Fatalf("got %v", err)
	}
}

// countingAT records calls so tests can assert AT is never reached.
type countingAT struct{ calls int }

func (c *countingAT) RegisterSeries(ctx context.Context, req at.RegisterRequest) (*at.RegisterResult, error) {
	c.calls++
	return &at.RegisterResult{ValidationCode: "AAJFJW8H89"}, nil
}

func (c *countingAT) ConsultSeries(ctx context.Context, username, password, seriesCode string) (*at.RegisterResult, error) {
	c.calls++
	return nil, errors.New("unused")
}

// Same series code already bound to GT must not report "registered" for GR, must not
// reach AT (orphan series there), and must not rewrite the GT row.
func TestRegisterSeries_CodeTakenByOtherDocType(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-demo-001"
	if err := db.UpsertTaxpayer(store.TaxpayerInput{
		StoreID: storeID, TaxRegistrationNumber: "519655290", LegalName: "Demo",
		AddressDetail: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}
	fake := &countingAT{}
	svc := service.New(db, nil, fake, filepath.Join(dir, "secure"), storeID)
	if err := svc.UpsertATCredentials(storeID, "519655290/1", "pw"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterSeries(context.Background(), storeID, "GR2026T01", "GT", 2026); err != nil {
		t.Fatal(err)
	}
	before := fake.calls

	_, err = svc.RegisterSeries(context.Background(), storeID, "GR2026T01", "GR", 2026)
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != service.ErrCodeSeriesCodeTaken {
		t.Fatalf("want series_code_taken, got %v", err)
	}
	if fake.calls != before {
		t.Fatalf("AT must not be called for a locally taken code (calls %d → %d)", before, fake.calls)
	}
	if got, err := db.SeriesDocTypeForCode(storeID, "GR2026T01"); err != nil || got != "GT" {
		t.Fatalf("GT row must stay GT: %q %v", got, err)
	}
	if err := db.UpsertActiveSeries(storeID, "GR", "GR2026T01", "XXXXXXXX", 2026); !errors.Is(err, store.ErrSeriesCodeOtherDocType) {
		t.Fatalf("store must refuse rewriting document_type: %v", err)
	}
}
