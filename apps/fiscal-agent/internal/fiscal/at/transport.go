package at

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// TransportAddress is AT AddressStructurePT (Country is always PT). Callers pass
// already-validated structured fields (domain.Address.ValidatePT); nothing is parsed here.
type TransportAddress struct {
	Detail     string
	City       string
	PostalCode string // NNNN-NNN
}

// TransportLine is one AT Line; UnitPrice is net of VAT.
type TransportLine struct {
	Description   string
	Quantity      string
	UnitOfMeasure string
	UnitPrice     string
}

// TransportDocument is the payload sent to AT before a GR/GT is committed locally.
// DocumentNumber must be the real number the document will keep ("GR SERIE/N").
type TransportDocument struct {
	Username              string // AT sub-user (NIF/n); never logged
	Password              string
	TaxRegistrationNumber string
	CompanyName           string
	CompanyAddress        TransportAddress
	DocumentNumber        string
	ATCUD                 string // validation_code-seq; AT warns "Erro na validação do ATCUD" without it
	MovementType          string // GR | GT
	MovementStatus        string // N
	MovementDate          string // YYYY-MM-DD
	MovementStartTime     string // YYYY-MM-DDTHH:MM:SS
	CustomerTaxID         string
	CustomerName          string
	CustomerAddress       TransportAddress
	ShipFrom              TransportAddress // load point (AddressFrom)
	ShipTo                TransportAddress // unload point (AddressTo)
	Lines                 []TransportLine
}

// TransportResult is AT's response after successful communication.
type TransportResult struct {
	ATDocCodeID string
	RawMessage  string
}

// TransportClient is the ONLY AT Documentos de Transporte surface (GR/GT).
// Must not be conflated with series Client (registarSerie).
type TransportClient interface {
	RegisterDocument(ctx context.Context, doc TransportDocument) (*TransportResult, error)
}

// NewTransportFromEnv builds transport client from FISCAL_AT_ENV (empty → mock; invalid → every call fails).
func NewTransportFromEnv() TransportClient {
	env, err := EnvFromProcess()
	if err != nil {
		return &misconfiguredClient{err: err}
	}
	switch env {
	case EnvTest, EnvProd:
		return &SOAPTransportClient{Env: env}
	default:
		return &MockTransportClient{}
	}
}

// MockTransportClient returns a deterministic fake ATDocCodeID (CI / default).
type MockTransportClient struct {
	Fail bool
}

func (m *MockTransportClient) RegisterDocument(ctx context.Context, doc TransportDocument) (*TransportResult, error) {
	_ = ctx
	if m != nil && m.Fail {
		return nil, fmt.Errorf("at: transport mock failure")
	}
	if strings.TrimSpace(doc.DocumentNumber) == "" {
		return nil, fmt.Errorf("at: document number required")
	}
	sum := sha256.Sum256([]byte(doc.DocumentNumber + "|" + doc.MovementType + "|" + doc.MovementStartTime))
	code := "MOCK" + strings.ToUpper(hex.EncodeToString(sum[:4]))
	return &TransportResult{ATDocCodeID: code, RawMessage: "mock ok " + time.Now().UTC().Format(time.RFC3339)}, nil
}
