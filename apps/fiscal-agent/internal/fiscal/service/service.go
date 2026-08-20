package service

import (
	"context"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// Store abstracts persistence for tests and service layer.
type Store interface {
	// Implemented by store/sqlite in a later step.
}

// Signer abstracts RSA-SHA1 signing (TPM/CNG or test key).
type Signer interface {
	Sign(payload string) (hashBase64 string, hashControl int, keyVersion int, err error)
}

// FiscalService is the application entry for issuance and reprint.
type FiscalService struct {
	store  Store
	signer Signer
}

// New returns a FiscalService. Dependencies wired at agent bootstrap.
func New(store Store, signer Signer) *FiscalService {
	return &FiscalService{store: store, signer: signer}
}

// IssueDocument signs a sale snapshot as FT (or FS when enabled).
func (s *FiscalService) IssueDocument(ctx context.Context, req domain.IssueRequest, docType domain.DocumentType) (*domain.IssueResult, error) {
	_ = ctx
	_ = req
	_ = docType
	return nil, ErrNotImplemented
}

// ErrNotImplemented marks scaffold APIs not yet wired.
var ErrNotImplemented = errNotImplemented{}

type errNotImplemented struct{}

func (errNotImplemented) Error() string { return "fiscal: not implemented" }
