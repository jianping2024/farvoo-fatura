package at

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ParseEnv is the ONLY FISCAL_AT_ENV parser. Empty → mock; mock|test|prod
// (case-insensitive); anything else is an error — never a silent fallback to mock.
func ParseEnv(raw string) (Env, error) {
	switch v := Env(strings.ToLower(strings.TrimSpace(raw))); v {
	case "":
		return EnvMock, nil
	case EnvMock, EnvTest, EnvProd:
		return v, nil
	default:
		return "", fmt.Errorf("at: invalid fiscal_at_env %q (want mock|test|prod; check config.json)", strings.TrimSpace(raw))
	}
}

// EnvFromProcess parses process FISCAL_AT_ENV (bridged from config.json by the Agent).
func EnvFromProcess() (Env, error) {
	return ParseEnv(os.Getenv("FISCAL_AT_ENV"))
}

// misconfiguredClient fails every series/transport call when FISCAL_AT_ENV is invalid.
type misconfiguredClient struct {
	err error
}

func (c *misconfiguredClient) RegisterSeries(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	return nil, c.err
}

func (c *misconfiguredClient) ConsultSeries(ctx context.Context, username, password, seriesCode string) (*RegisterResult, error) {
	return nil, c.err
}

func (c *misconfiguredClient) RegisterDocument(ctx context.Context, doc TransportDocument) (*TransportResult, error) {
	return nil, c.err
}
