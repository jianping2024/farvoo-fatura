// Package at talks to Autoridade Tributária Series SOAP (or mock).
package at

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Env is AT connectivity mode.
type Env string

const (
	EnvMock Env = "mock"
	EnvTest Env = "test"
	EnvProd Env = "prod"
)

// RegisterRequest is input for registarSerie.
type RegisterRequest struct {
	Username           string // NIF/nn
	Password           string
	SeriesCode         string // serie identifier without type prefix handling — full series_code
	DocumentType       string // FT
	FiscalYear         int
	SoftwareCertNumber int
}

// RegisterResult holds AT validation code.
type RegisterResult struct {
	ValidationCode string
	RawMessage     string
}

// Client is the ONLY AT series client surface.
type Client interface {
	RegisterSeries(ctx context.Context, req RegisterRequest) (*RegisterResult, error)
	ConsultSeries(ctx context.Context, username, password, seriesCode string) (*RegisterResult, error)
}

// NewFromEnv builds client from FISCAL_AT_ENV (default mock).
func NewFromEnv() Client {
	switch Env(strings.ToLower(strings.TrimSpace(os.Getenv("FISCAL_AT_ENV")))) {
	case EnvTest, EnvProd:
		return &SOAPClient{Env: Env(strings.ToLower(os.Getenv("FISCAL_AT_ENV")))}
	default:
		code := os.Getenv("FISCAL_MOCK_VALIDATION_CODE")
		if code == "" {
			code = "CSDF7T5H"
		}
		return &MockClient{ValidationCode: code}
	}
}

// MockClient returns a fixed 8-char validation code.
type MockClient struct {
	ValidationCode string
	Fail           bool
}

func (m *MockClient) RegisterSeries(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	_ = ctx
	if m.Fail {
		return nil, fmt.Errorf("at: mock failure")
	}
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("at: credentials required")
	}
	if len(m.ValidationCode) != 8 {
		return nil, fmt.Errorf("at: mock validation code must be 8 chars")
	}
	return &RegisterResult{ValidationCode: m.ValidationCode, RawMessage: "mock ok"}, nil
}

func (m *MockClient) ConsultSeries(ctx context.Context, username, password, seriesCode string) (*RegisterResult, error) {
	return m.RegisterSeries(ctx, RegisterRequest{Username: username, Password: password, SeriesCode: seriesCode})
}

// SOAPClient is a stub for real AT endpoints (M1: returns clear error until certs wired).
type SOAPClient struct {
	Env Env
}

func (s *SOAPClient) RegisterSeries(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	_ = ctx
	_ = req
	return nil, fmt.Errorf("at: SOAP %s not configured (use FISCAL_AT_ENV=mock until certificates are available)", s.Env)
}

func (s *SOAPClient) ConsultSeries(ctx context.Context, username, password, seriesCode string) (*RegisterResult, error) {
	_ = ctx
	_ = username
	_ = password
	_ = seriesCode
	return nil, fmt.Errorf("at: SOAP %s not configured (use FISCAL_AT_ENV=mock until certificates are available)", s.Env)
}
