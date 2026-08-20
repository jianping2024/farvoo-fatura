package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/protect"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/google/uuid"
)

// API error codes (stable).
const (
	ErrCodeTaxpayerMissing = "taxpayer_missing"
	ErrCodeATCredsMissing  = "at_credentials_missing"
	ErrCodeSeriesMissing   = "series_missing"
	ErrCodeSignerNotReady  = "signer_not_ready"
	ErrCodeATSOAPFailed    = "at_soap_failed"
)

// CodedError carries a stable error code for HTTP.
type CodedError struct {
	Code string
	Msg  string
}

func (e *CodedError) Error() string { return e.Msg }

func coded(code, msg string) error { return &CodedError{Code: code, Msg: msg} }

// FiscalService orchestrates setup + issuance.
type FiscalService struct {
	db      *store.DB
	signer  store.Signer
	at      at.Client
	dataDir string
	storeID string
}

// New returns a FiscalService. signer may be nil until Activate; IssueDocument loads from DB if nil.
func New(db *store.DB, sig store.Signer, atClient at.Client, dataDir, storeID string) *FiscalService {
	if atClient == nil {
		atClient = at.NewFromEnv()
	}
	return &FiscalService{db: db, signer: sig, at: atClient, dataDir: dataDir, storeID: storeID}
}

// SetSigner replaces the in-memory signer after activation.
func (s *FiscalService) SetSigner(sig store.Signer) { s.signer = sig }

func (s *FiscalService) ensureSigner() (store.Signer, error) {
	if s.signer != nil {
		return s.signer, nil
	}
	wrapped, _, ver, err := s.db.ActiveSigningKey()
	if err != nil {
		return nil, coded(ErrCodeSignerNotReady, "signing key not activated")
	}
	sig, err := signer.NewUnwrappingSigner(s.dataDir, wrapped, ver)
	if err != nil {
		return nil, coded(ErrCodeSignerNotReady, err.Error())
	}
	s.signer = sig
	return sig, nil
}

// UpsertTaxpayer is the ONLY taxpayer setup entry.
func (s *FiscalService) UpsertTaxpayer(p store.TaxpayerInput) error {
	if p.StoreID == "" {
		p.StoreID = s.storeID
	}
	if p.Country == "" {
		p.Country = "PT"
	}
	if p.Timezone == "" {
		p.Timezone = "Europe/Lisbon"
	}
	if p.SoftwareCertificateNumber == "" {
		p.SoftwareCertificateNumber = "0"
	}
	if p.TaxRegistrationNumber == "" || p.LegalName == "" || p.AddressDetail == "" || p.City == "" || p.PostalCode == "" {
		return fmt.Errorf("fiscal: taxpayer required fields missing")
	}
	if err := s.db.EnsureConsumidorFinal(); err != nil {
		return err
	}
	return s.db.UpsertTaxpayer(p)
}

// UpsertATCredentials seals and stores AT password — ONLY credentials write via service.
func (s *FiscalService) UpsertATCredentials(storeID, username, password string) error {
	if storeID == "" {
		storeID = s.storeID
	}
	if username == "" || password == "" {
		return fmt.Errorf("fiscal: username and password required")
	}
	ct, meta, err := protect.Seal(s.dataDir, []byte(password))
	if err != nil {
		return err
	}
	return s.db.UpsertATCredentials(storeID, username, ct, meta)
}

// RegisterSeries calls AT (or mock) and persists ACTIVE series — ONLY series registration entry.
func (s *FiscalService) RegisterSeries(ctx context.Context, storeID, seriesCode, docType string, fiscalYear int) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	if docType == "" {
		docType = "FT"
	}
	if fiscalYear == 0 {
		fiscalYear = time.Now().Year()
	}
	if seriesCode == "" {
		seriesCode = fmt.Sprintf("%s%dDEMO01", docType, fiscalYear)
	}
	st, err := s.db.GetSetupStatus(storeID)
	if err != nil {
		return nil, err
	}
	if !st.TaxpayerOK {
		return nil, coded(ErrCodeTaxpayerMissing, "configure taxpayer first")
	}
	cred, err := s.db.GetATCredentials(storeID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeATCredsMissing, "configure AT credentials first")
	}
	if err != nil {
		return nil, err
	}
	pass, err := protect.Open(s.dataDir, cred.PasswordCiphertext, cred.WrapMeta)
	if err != nil {
		return nil, err
	}
	var certStr string
	_ = s.db.SQL.QueryRow(`SELECT software_certificate_number FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&certStr)
	certN, _ := strconv.Atoi(certStr)

	res, err := s.at.RegisterSeries(ctx, at.RegisterRequest{
		Username: cred.Username, Password: string(pass),
		SeriesCode: seriesCode, DocumentType: docType, FiscalYear: fiscalYear, SoftwareCertNumber: certN,
	})
	if err != nil {
		_ = s.db.MarkATCredentialsError(storeID, err.Error())
		return nil, coded(ErrCodeATSOAPFailed, err.Error())
	}
	if err := s.db.UpsertActiveSeries(storeID, docType, seriesCode, res.ValidationCode, fiscalYear); err != nil {
		return nil, err
	}
	_ = s.db.MarkATCredentialsOK(storeID)
	return s.db.GetSetupStatus(storeID)
}

// ActivateFiscal provisions device key + wraps product PEM — ONLY activation entry.
func (s *FiscalService) ActivateFiscal(storeID, productPEM string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "1" {
		return nil, fmt.Errorf("fiscal: local provision disabled (set FISCAL_ALLOW_LOCAL_PROVISION=1 for M1)")
	}
	productPEM = strings.TrimSpace(productPEM)
	if productPEM == "" {
		return nil, fmt.Errorf("fiscal: product_private_key_pem required")
	}
	var nif string
	err := s.db.SQL.QueryRow(`SELECT tax_registration_number FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&nif)
	if err != nil {
		return nil, coded(ErrCodeTaxpayerMissing, "configure taxpayer first")
	}

	dev, err := signer.GenerateDeviceKey()
	if err != nil {
		return nil, err
	}
	if err := signer.SaveDeviceKey(s.dataDir, dev); err != nil {
		return nil, err
	}
	wrapped, err := signer.WrapProductPEM(&dev.Private.PublicKey, []byte(productPEM))
	if err != nil {
		return nil, err
	}
	inner, err := signer.LoadPEM([]byte(productPEM), 1)
	if err != nil {
		return nil, fmt.Errorf("fiscal: product PEM: %w", err)
	}
	pub, err := inner.PublicKeyPEM()
	if err != nil {
		return nil, err
	}
	if err := s.db.SaveActivation(store.ActivationInput{
		InstallationID:     "inst-" + uuid.NewString(),
		StoreID:            storeID,
		TaxpayerNIF:        nif,
		DeviceID:           "device-local",
		DevicePublicKey:    dev.PublicPEM,
		KeyProtectionLevel: "SOFTWARE",
		KeyVersion:         1,
		PublicKeyPEM:       pub,
		WrappedPrivateKey:  wrapped,
	}); err != nil {
		return nil, err
	}
	sig, err := signer.NewUnwrappingSigner(s.dataDir, wrapped, 1)
	if err != nil {
		return nil, err
	}
	s.signer = sig
	return s.db.GetSetupStatus(storeID)
}

// UpsertOperator is the ONLY operator setup entry for M1.
func (s *FiscalService) UpsertOperator(id, storeID, role, name string) error {
	if storeID == "" {
		storeID = s.storeID
	}
	if id == "" {
		id = "op-demo-cashier"
	}
	if role == "" {
		role = "cashier"
	}
	if name == "" {
		name = "Cashier"
	}
	return s.db.UpsertOperator(id, storeID, role, name, "")
}

// SetupStatus proxies store.
func (s *FiscalService) SetupStatus(storeID string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	return s.db.GetSetupStatus(storeID)
}

// IssueDocument signs a sale snapshot as FT (P0).
func (s *FiscalService) IssueDocument(ctx context.Context, req domain.IssueRequest, docType domain.DocumentType) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, fmt.Errorf("fiscal: request_id required")
	}
	if req.StoreID == "" {
		req.StoreID = s.storeID
	}
	if req.OperatorID == "" {
		return nil, fmt.Errorf("fiscal: operator_id required")
	}
	if len(req.Snapshot.Lines) == 0 {
		return nil, fmt.Errorf("fiscal: lines required")
	}
	st, err := s.db.GetSetupStatus(req.StoreID)
	if err != nil {
		return nil, err
	}
	if !st.SeriesOK {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE FT series with validation_code")
	}
	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}
	rec, err := s.db.IssueFT(ctx, sig, store.IssueParams{
		StoreID: req.StoreID, RequestID: req.RequestID, DocType: docType,
		Snapshot: req.Snapshot, OperatorID: req.OperatorID,
	})
	if err != nil {
		return nil, err
	}
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: rec.IdempotentHit,
	}, nil
}

// GetByRequestID looks up a prior issue.
func (s *FiscalService) GetByRequestID(storeID, requestID string) (*domain.IssueResult, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	rec, err := s.db.GetByRequestID(storeID, requestID)
	if err != nil {
		return nil, err
	}
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: true,
	}, nil
}

// GetPrintJob proxies store.
func (s *FiscalService) GetPrintJob(id string) (*store.PrintJobView, error) {
	return s.db.GetPrintJob(id)
}
