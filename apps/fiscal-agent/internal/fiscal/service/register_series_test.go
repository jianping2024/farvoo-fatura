package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

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
