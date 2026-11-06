// Package software is the ONLY source of the invoicing program's identity: SAF-T Header 1.14–1.16
// (ProductCompanyTaxID, SoftwareCertificateNumber, ProductID, ProductVersion), the certificate
// number stored and printed on every document, and the one sent with AT series registration.
// It belongs to the software (Portaria 363/2010), not to a store's taxpayer settings.
package software

import "strings"

const (
	// ProductName is the commercial name of the application.
	ProductName = "Farvoo"
	// ProducerName is the legal name of the software producer.
	ProducerName = "FAINA VAIDOSA, LDA"
	// ProducerTaxID is the producer's Portuguese NIF (SAF-T 1.14 ProductCompanyTaxID).
	ProducerTaxID = "519655290"
	// CertificateNumber is the AT software certificate number; "0" until the program is certified.
	CertificateNumber = "0"
)

// ProductID is SAF-T 1.16: "application name/producer name".
func ProductID() string { return ProductName + "/" + ProducerName }

var version = "dev"

// SetVersion records the build version (apps/fiscal-agent/VERSION via -ldflags main.Version).
// Called once by bootstrap; dev builds keep "dev".
func SetVersion(v string) {
	if v = strings.TrimSpace(v); v != "" {
		version = v
	}
}

// Version is SAF-T 1.16 ProductVersion.
func Version() string { return version }
