package at

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// TransportDocument is the payload sent to AT before a GR/GT is committed locally.
type TransportDocument struct {
	TaxRegistrationNumber string
	CompanyName           string
	DocumentNumber        string
	MovementType          string // GR | GT
	MovementStatus        string // N
	MovementDate          string // YYYY-MM-DD
	MovementStartTime     string // YYYY-MM-DDTHH:MM:SS
	CustomerTaxID         string
	CustomerName          string
	ShipFromAddress       string
	ShipToAddress         string
	LineDescriptions      []string
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

// NewTransportFromEnv builds transport client from FISCAL_AT_ENV (default mock).
func NewTransportFromEnv() TransportClient {
	switch Env(strings.ToLower(strings.TrimSpace(os.Getenv("FISCAL_AT_ENV")))) {
	case EnvTest, EnvProd:
		return &SOAPTransportClient{Env: Env(strings.ToLower(os.Getenv("FISCAL_AT_ENV")))}
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

// SOAPTransportClient is the real AT transport webservice stub until live auth is wired.
// P0: returns a clear error so IssueGR fails closed when FISCAL_AT_ENV=test|prod without full SOAP.
type SOAPTransportClient struct {
	Env Env
}

func (c *SOAPTransportClient) RegisterDocument(ctx context.Context, doc TransportDocument) (*TransportResult, error) {
	_ = ctx
	_ = doc
	return nil, fmt.Errorf("at: transport SOAP not wired for env %s (use mock or complete live client)", c.Env)
}
