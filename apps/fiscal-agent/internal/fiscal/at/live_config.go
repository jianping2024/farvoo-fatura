package at

import (
	"os"
	"strings"
)

// LiveMaterials holds paths/credentials for AT test/prod SOAP (not used when Env=mock).
// ONLY LiveMaterialsFromEnv reads these names (+ legacy aliases).
type LiveMaterials struct {
	ClientCertPath     string
	ClientCertPass     string
	PasswordPubKeyPath string
	ClientUser         string // optional override of at_credentials.username
	ClientPass         string // optional override of sealed password
	SeriesURL          string
	TransportURL       string
}

// LiveMaterialsFromEnv is the ONLY env/config surface for AT live material paths.
// Prefers FISCAL_AT_CLIENT_* / FISCAL_AT_PASSWORD_PUBKEY; falls back to legacy
// FISCAL_AT_TRANSPORT_* aliases from fiscal-gr §8 for one transition period.
func LiveMaterialsFromEnv() LiveMaterials {
	return LiveMaterials{
		ClientCertPath: firstNonEmpty(
			os.Getenv("FISCAL_AT_CLIENT_CERT"),
			os.Getenv("FISCAL_AT_TRANSPORT_CERT"),
		),
		ClientCertPass: firstNonEmpty(
			os.Getenv("FISCAL_AT_CLIENT_CERT_PASS"),
			os.Getenv("FISCAL_AT_TRANSPORT_CERT_PASS"),
		),
		PasswordPubKeyPath: firstNonEmpty(
			os.Getenv("FISCAL_AT_PASSWORD_PUBKEY"),
			os.Getenv("FISCAL_AT_TRANSPORT_PUBKEY"),
		),
		ClientUser: firstNonEmpty(
			os.Getenv("FISCAL_AT_CLIENT_USER"),
			os.Getenv("FISCAL_AT_TRANSPORT_USER"),
		),
		ClientPass: firstNonEmpty(
			os.Getenv("FISCAL_AT_CLIENT_PASS"),
			os.Getenv("FISCAL_AT_TRANSPORT_PASS"),
		),
		SeriesURL:    strings.TrimSpace(os.Getenv("FISCAL_AT_SERIES_URL")),
		TransportURL: firstNonEmpty(os.Getenv("FISCAL_AT_TRANSPORT_URL")),
	}
}

// MissingRequired reports empty critical paths (cert + pubkey). Passphrase may be empty only if pfx allows it (ours does not).
func (m LiveMaterials) MissingRequired() []string {
	var miss []string
	if strings.TrimSpace(m.ClientCertPath) == "" {
		miss = append(miss, "FISCAL_AT_CLIENT_CERT")
	}
	if strings.TrimSpace(m.ClientCertPass) == "" {
		miss = append(miss, "FISCAL_AT_CLIENT_CERT_PASS")
	}
	if strings.TrimSpace(m.PasswordPubKeyPath) == "" {
		miss = append(miss, "FISCAL_AT_PASSWORD_PUBKEY")
	}
	return miss
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
