package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestOpsFiscalEntitled_LocalProvisionBypass(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "1")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := service.New(db, nil, nil, dir, "store-uat")
	entitled, _, paired := svc.OpsFiscalEntitled(context.Background())
	if !entitled {
		t.Fatal("local provision must entitle")
	}
	if paired {
		t.Fatal("unpaired cloud must report paired=false")
	}
	id, err := svc.BootstrapOwner(context.Background(), "store-uat", "Admin", "123456")
	if err != nil || id == "" {
		t.Fatalf("bootstrap: id=%q err=%v", id, err)
	}
}

func TestBootstrapOwner_RejectsWithoutOpsEntitlement(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "0")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := service.New(db, nil, nil, dir, "store-blocked")
	_, err = svc.BootstrapOwner(context.Background(), "store-blocked", "Admin", "123456")
	if err == nil {
		t.Fatal("expected bootstrap reject without Ops entitlement")
	}
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != service.ErrCodeCloudNotPaired {
		t.Fatalf("want cloud_not_paired, got %v", err)
	}
	entitled, _, paired := svc.OpsFiscalEntitled(context.Background())
	if entitled || paired {
		t.Fatalf("entitled=%v paired=%v", entitled, paired)
	}
}
