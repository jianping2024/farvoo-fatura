package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestActivateFromCloud_RequiresCloudPairing(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-cloud-pair"
	if err := db.UpsertTaxpayer(store.TaxpayerInput{
		StoreID: storeID, TaxRegistrationNumber: "517535009", LegalName: "Demo",
		AddressDetail: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}
	svc := service.New(db, nil, nil, dir, storeID)
	_, err = svc.ActivateFromCloud(context.Background(), storeID)
	if err == nil {
		t.Fatal("expected error without cloud pairing")
	}
	var coded *service.CodedError
	if !errors.As(err, &coded) || coded.Code != service.ErrCodeCloudNotPaired {
		t.Fatalf("want cloud_not_paired, got %v", err)
	}
}

func TestSetupStatus_CloudPairedOK(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-status"
	svc := service.New(db, nil, nil, dir, storeID)
	st, err := svc.SetupStatus(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if st.CloudPairedOK {
		t.Fatal("expected cloud_paired_ok false")
	}
	svc.SetCloudProvision(service.CloudProvision{APIBase: "http://127.0.0.1:3000", JWT: "jwt", DeviceID: "dev-1"})
	st, err = svc.SetupStatus(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.CloudPairedOK {
		t.Fatal("expected cloud_paired_ok true")
	}
}
