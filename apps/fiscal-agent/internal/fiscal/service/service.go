package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/fiscalsigning"
	"farvoo-fiscal-agent/internal/fiscal/protect"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/google/uuid"
)

// API error codes (stable).
const (
	ErrCodeTaxpayerMissing      = "taxpayer_missing"
	ErrCodeATCredsMissing       = "at_credentials_missing"
	ErrCodeSeriesMissing        = "series_missing"
	ErrCodeSeriesAlreadyActive  = "series_already_active"
	ErrCodeSignerNotReady       = "signer_not_ready"
	ErrCodeOpsActivatePending   = "ops_activate_pending"
	ErrCodeATSOAPFailed         = "at_soap_failed"
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
	cloud   CloudProvision
}

// CloudProvision is Farvoo API identity for Ops-wrapped product key pull.
type CloudProvision struct {
	APIBase  string
	JWT      string
	DeviceID string
}

// New returns a FiscalService. signer may be nil until Activate; IssueDocument loads from DB if nil.
func New(db *store.DB, sig store.Signer, atClient at.Client, dataDir, storeID string) *FiscalService {
	if atClient == nil {
		atClient = at.NewFromEnv()
	}
	return &FiscalService{db: db, signer: sig, at: atClient, dataDir: dataDir, storeID: storeID}
}

// SetCloudProvision configures Agent→Farvoo fiscal-signing client (ONLY cloud provision config entry).
func (s *FiscalService) SetCloudProvision(c CloudProvision) {
	if s == nil {
		return
	}
	s.cloud = c
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

// SeriesRegisterResult is returned by RegisterSeries (ONLY series registration entry).
type SeriesRegisterResult struct {
	Status        *store.SetupStatus
	IdempotentHit bool
	SeriesCode    string
	DocumentType  string
}

// RegisterSeries calls AT (or mock) and persists ACTIVE series — ONLY series registration entry.
// Same ACTIVE series_code → idempotent (no AT). Different ACTIVE code for same document_type → reject.
func (s *FiscalService) RegisterSeries(ctx context.Context, storeID, seriesCode, docType string, fiscalYear int) (*SeriesRegisterResult, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	docType = strings.ToUpper(strings.TrimSpace(docType))
	if docType == "" {
		docType = "FT"
	}
	if fiscalYear == 0 {
		fiscalYear = time.Now().Year()
	}
	seriesCode = strings.TrimSpace(seriesCode)
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

	hasSame, err := s.db.ActiveSeriesHasCode(storeID, seriesCode)
	if err != nil {
		return nil, err
	}
	if hasSame {
		return &SeriesRegisterResult{Status: st, IdempotentHit: true, SeriesCode: seriesCode, DocumentType: docType}, nil
	}

	existingCode, err := s.db.ActiveSeriesCodeForDocType(storeID, docType)
	if err == nil && !strings.EqualFold(existingCode, seriesCode) {
		return nil, coded(ErrCodeSeriesAlreadyActive,
			fmt.Sprintf("%s series already active (%s); refuse registering %s", docType, existingCode, seriesCode))
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
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
	st, err = s.db.GetSetupStatus(storeID)
	if err != nil {
		return nil, err
	}
	return &SeriesRegisterResult{Status: st, IdempotentHit: false, SeriesCode: seriesCode, DocumentType: docType}, nil
}

// ActivateFiscal provisions device key + wraps product PEM — ONLY local-PEM activation entry (UAT).
func (s *FiscalService) ActivateFiscal(storeID, productPEM string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "1" {
		return nil, fmt.Errorf("fiscal: local provision disabled (use Ops activate + activate-from-cloud)")
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

	dev, err := s.ensureDeviceKey()
	if err != nil {
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
	deviceID := strings.TrimSpace(s.cloud.DeviceID)
	if deviceID == "" {
		deviceID = "device-local"
	}
	if err := s.db.SaveActivation(store.ActivationInput{
		InstallationID:     "inst-" + uuid.NewString(),
		StoreID:            storeID,
		TaxpayerNIF:        nif,
		DeviceID:           deviceID,
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

// RegisterCloudDevice ensures B' and POSTs register — ONLY device-register orchestration.
func (s *FiscalService) RegisterCloudDevice(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	if strings.TrimSpace(s.cloud.APIBase) == "" || strings.TrimSpace(s.cloud.JWT) == "" || strings.TrimSpace(s.cloud.DeviceID) == "" {
		return fmt.Errorf("fiscal: cloud register requires pairing")
	}
	dev, err := s.ensureDeviceKey()
	if err != nil {
		return err
	}
	cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
	_, _, err = cli.RegisterDevicePublicKey(ctx, dev.PublicPEM)
	return err
}

// ActivateFromCloud registers B' and pulls Ops-wrapped C — ONLY cloud activation entry.
func (s *FiscalService) ActivateFromCloud(ctx context.Context, storeID string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	if err := s.RegisterCloudDevice(ctx); err != nil {
		return nil, err
	}
	var nif string
	err := s.db.SQL.QueryRow(`SELECT tax_registration_number FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&nif)
	if err != nil {
		return nil, coded(ErrCodeTaxpayerMissing, "configure taxpayer first")
	}

	dev, err := s.ensureDeviceKey()
	if err != nil {
		return nil, err
	}
	cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
	bundle, err := cli.PullProvision(ctx)
	if err != nil {
		if errors.Is(err, fiscalsigning.ErrNotActive) {
			return nil, coded(ErrCodeOpsActivatePending, "ops has not activated fiscal signing yet")
		}
		return nil, err
	}
	if err := s.db.SaveActivation(store.ActivationInput{
		InstallationID:     bundle.InstallationID,
		StoreID:            storeID,
		TaxpayerNIF:        nif,
		DeviceID:           s.cloud.DeviceID,
		DevicePublicKey:    dev.PublicPEM,
		KeyProtectionLevel: "SOFTWARE",
		KeyVersion:         bundle.SigningKeyVersion,
		PublicKeyPEM:       bundle.ProductPublicKeyPEM,
		WrappedPrivateKey:  bundle.WrappedPrivateKey,
	}); err != nil {
		return nil, err
	}
	sig, err := signer.NewUnwrappingSigner(s.dataDir, bundle.WrappedPrivateKey, bundle.SigningKeyVersion)
	if err != nil {
		return nil, err
	}
	s.signer = sig
	return s.db.GetSetupStatus(storeID)
}

// ensureDeviceKey loads or creates the local device keypair — ONLY device-key ensure path.
func (s *FiscalService) ensureDeviceKey() (*signer.DeviceBundle, error) {
	if dev, err := signer.LoadDeviceKey(s.dataDir); err == nil {
		return dev, nil
	}
	dev, err := signer.GenerateDeviceKey()
	if err != nil {
		return nil, err
	}
	if err := signer.SaveDeviceKey(s.dataDir, dev); err != nil {
		return nil, err
	}
	return dev, nil
}

// TryPullCloudProvisionIfNeeded reconciles cloud signing on Agent start (ONLY startup cloud sync).
// - Local active + cloud not_active/revoked → ClearLocalActivation (no per-issue cloud check).
// - Local not active + taxpayer OK → register + pull when Ops already activated.
func (s *FiscalService) TryPullCloudProvisionIfNeeded(ctx context.Context) {
	if s == nil || s.db == nil {
		return
	}
	if strings.TrimSpace(s.cloud.APIBase) == "" || strings.TrimSpace(s.cloud.JWT) == "" {
		return
	}
	_ = s.RegisterCloudDevice(ctx)
	st, err := s.db.GetSetupStatus(s.storeID)
	if err != nil {
		return
	}
	if st.ActivatedOK {
		cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
		_, pullErr := cli.PullProvision(ctx)
		if errors.Is(pullErr, fiscalsigning.ErrNotActive) {
			if clearErr := s.db.ClearLocalActivation(); clearErr == nil {
				s.signer = nil
			}
		}
		return
	}
	if !st.TaxpayerOK {
		return
	}
	_, _ = s.ActivateFromCloud(ctx, s.storeID)
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

// SetOperatorCanIssueNC is the ONLY service entry for operators.can_issue_nc updates.
func (s *FiscalService) SetOperatorCanIssueNC(storeID, operatorID string, canIssue bool) error {
	if storeID == "" {
		storeID = s.storeID
	}
	if operatorID == "" {
		operatorID = "op-demo-cashier"
	}
	return s.db.SetOperatorCanIssueNC(storeID, operatorID, canIssue)
}

// SetupStatus proxies store.
func (s *FiscalService) SetupStatus(storeID string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	return s.db.GetSetupStatus(storeID)
}

// IssueDocument signs a sale snapshot as FT / FS / FR.
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
	switch docType {
	case domain.DocumentFT, domain.DocumentFS, domain.DocumentFR:
	default:
		return nil, coded(ErrCodeValidationFailed, "document_type must be FT, FS, or FR")
	}
	ok, err := s.db.HasActiveSeries(req.StoreID, string(docType))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, coded(ErrCodeSeriesMissing, fmt.Sprintf("no ACTIVE %s series with validation_code", docType))
	}
	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}
	rec, err := s.db.IssueFT(ctx, sig, store.IssueParams{
		StoreID: req.StoreID, RequestID: req.RequestID, DocType: docType,
		Snapshot: req.Snapshot, OperatorID: req.OperatorID, StationID: req.StationID,
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

// ListBillDrafts returns local bill sync drafts (read model).
func (s *FiscalService) ListBillDrafts(limit int) ([]store.BillSyncDraft, error) {
	return s.db.ListBillDrafts(limit)
}

// IssueBillDraftInput is the ONLY input shape for IssueFromBillDraft.
type IssueBillDraftInput struct {
	DraftID            string
	DocumentType       string // empty → domain.DefaultSaleDocumentType (FS)
	OperatorID         string
	Mode               string // whole_table | person; empty → whole_table
	ScopeID            string
	StationID          string // required: station_printers key for ORIGINAL print
	CustomerNIF        string
	CustomerName       string
	AllocationRevision *int64 // person: required OCC; must match draft.allocation_revision
}

// BillDraftDetail is GET /bill-drafts/{id} read model.
type BillDraftDetail struct {
	Draft               store.BillSyncDraft  `json:"draft"`
	Payload             billsync.Snapshot    `json:"payload"`
	Allocation          billsync.Allocation  `json:"allocation"`
	AllocationRevision  int64                `json:"allocation_revision"`
	IssuedScopes        []store.SignedSaleScope `json:"issued_scopes"`
	Remaining           map[string]string    `json:"remaining,omitempty"` // line_key → qty label
}

// GetBillDraftDetail loads one draft + payload + allocation + issued scopes (tax DB).
func (s *FiscalService) GetBillDraftDetail(draftID string) (*BillDraftDetail, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	draft, err := s.db.GetBillDraftByID(strings.TrimSpace(draftID))
	if err != nil {
		return nil, err
	}
	var snap billsync.Snapshot
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &snap); err != nil {
		return nil, fmt.Errorf("fiscal: draft payload: %w", err)
	}
	if strings.TrimSpace(snap.SourceSaleID) == "" {
		snap.SourceSaleID = draft.SourceSaleID
	}
	if err := billsync.FreezeSourceLines(&snap); err != nil && len(snap.SourceLines) == 0 {
		return nil, err
	}
	alloc, err := billsync.ParseAllocationJSON(draft.AllocationJSON)
	if err != nil {
		return nil, err
	}
	scopes, err := s.db.ListSignedSaleScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
	if err != nil {
		return nil, err
	}
	remMap := map[string]string{}
	if rem, err := billsync.RemainingPool(snap, alloc); err == nil {
		for k, q := range rem {
			remMap[k] = billsync.FormatRational(q)
		}
	}
	return &BillDraftDetail{
		Draft: *draft, Payload: snap, Allocation: alloc,
		AllocationRevision: draft.AllocationRevision,
		IssuedScopes: scopes, Remaining: remMap,
	}, nil
}

// SaveBillDraftAllocationInput is the ONLY input for SaveBillDraftAllocation.
type SaveBillDraftAllocationInput struct {
	DraftID            string
	ExpectedRevision   int64
	Allocation         billsync.Allocation
}

// SaveBillDraftAllocation is the ONLY service entry that persists local by-item allocation (OCC).
func (s *FiscalService) SaveBillDraftAllocation(in SaveBillDraftAllocationInput) (*BillDraftDetail, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	draft, err := s.db.GetBillDraftByID(strings.TrimSpace(in.DraftID))
	if err != nil {
		return nil, err
	}
	if draft.Status != store.BillDraftOpen {
		return nil, coded("draft_not_open", "draft not open")
	}
	var snap billsync.Snapshot
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &snap); err != nil {
		return nil, fmt.Errorf("fiscal: draft payload: %w", err)
	}
	if strings.TrimSpace(snap.SourceSaleID) == "" {
		snap.SourceSaleID = draft.SourceSaleID
	}
	if err := billsync.FreezeSourceLines(&snap); err != nil && len(snap.SourceLines) == 0 {
		return nil, err
	}
	// Lock issued people: cannot drop or mutate their shares.
	existing, err := s.db.ListSignedSaleScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
	if err != nil {
		return nil, err
	}
	issued := map[string]bool{}
	for _, sc := range existing {
		if strings.TrimSpace(sc.ScopeType) == "person" {
			issued[strings.TrimSpace(sc.ScopeID)] = true
		}
	}
	prev, err := billsync.ParseAllocationJSON(draft.AllocationJSON)
	if err != nil {
		return nil, err
	}
	billsync.NormalizeAllocation(&in.Allocation)
	prevByScope := map[string]billsync.AllocPerson{}
	for _, p := range prev.People {
		prevByScope[strings.TrimSpace(p.ScopeID)] = p
	}
	for _, p := range in.Allocation.People {
		sid := strings.TrimSpace(p.ScopeID)
		if issued[sid] {
			old := prevByScope[sid]
			if !allocPersonEqual(old, p) {
				return nil, coded("validation_failed", "cannot edit issued person allocation")
			}
		}
	}
	for sid := range issued {
		if _, ok := findPerson(in.Allocation, sid); !ok {
			return nil, coded("validation_failed", "cannot remove issued person from allocation")
		}
	}
	if err := billsync.ValidateAllocation(snap, in.Allocation); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(in.Allocation)
	if err != nil {
		return nil, err
	}
	updated, err := s.db.SaveBillDraftAllocation(draft.ID, in.ExpectedRevision, string(raw))
	if err != nil {
		if errors.Is(err, store.ErrAllocationConflict) {
			return nil, coded("allocation_conflict", "allocation was updated elsewhere; refresh and retry")
		}
		return nil, err
	}
	_ = updated
	return s.GetBillDraftDetail(draft.ID)
}

func findPerson(a billsync.Allocation, scopeID string) (billsync.AllocPerson, bool) {
	for _, p := range a.People {
		if strings.TrimSpace(p.ScopeID) == scopeID {
			return p, true
		}
	}
	return billsync.AllocPerson{}, false
}

func allocPersonEqual(a, b billsync.AllocPerson) bool {
	if strings.TrimSpace(a.Name) != strings.TrimSpace(b.Name) || len(a.Shares) != len(b.Shares) {
		return false
	}
	type key struct {
		k string
		n, d int64
	}
	ma := map[key]bool{}
	for _, sh := range a.Shares {
		q := billsync.NormalizeRational(sh.Qty)
		ma[key{sh.LineKey, q.Num, q.Den}] = true
	}
	for _, sh := range b.Shares {
		q := billsync.NormalizeRational(sh.Qty)
		if !ma[key{sh.LineKey, q.Num, q.Den}] {
			return false
		}
	}
	return true
}

// DiscardBillDrafts is the ONLY user discard entry: hard-delete drafts for the sale; never deletes invoices.
func (s *FiscalService) DiscardBillDrafts(draftID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service not configured")
	}
	draft, err := s.db.GetBillDraftByID(strings.TrimSpace(draftID))
	if err != nil {
		return err
	}
	return s.db.DeleteBillDraftsBySale(draft.SourceSaleID)
}

// IssueFromBillDraft is the ONLY entry that issues FT from a bill_sync_drafts row.
// whole_table → DraftToSaleSnapshot; person → DraftPersonFromAllocation (ONLY person builder).
func (s *FiscalService) IssueFromBillDraft(ctx context.Context, in IssueBillDraftInput) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	draftID := strings.TrimSpace(in.DraftID)
	operatorID := strings.TrimSpace(in.OperatorID)
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = "whole_table"
	}
	if draftID == "" {
		return nil, fmt.Errorf("fiscal: draft id required")
	}
	if operatorID == "" {
		return nil, fmt.Errorf("fiscal: operator_id required")
	}
	stationID := strings.TrimSpace(in.StationID)
	if stationID == "" {
		return nil, coded("validation_failed", "station_id required")
	}
	if mode != "whole_table" && mode != "person" {
		return nil, coded("validation_failed", fmt.Sprintf("mode %q invalid", mode))
	}

	draft, err := s.db.GetBillDraftByID(draftID)
	if err != nil {
		return nil, err
	}
	if draft.Status != store.BillDraftOpen {
		return nil, coded("draft_not_open", fmt.Sprintf("draft status %q is not open", draft.Status))
	}
	var snap billsync.Snapshot
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &snap); err != nil {
		return nil, fmt.Errorf("fiscal: draft payload: %w", err)
	}
	if strings.TrimSpace(snap.SourceSaleID) == "" {
		snap.SourceSaleID = draft.SourceSaleID
	}
	if strings.TrimSpace(snap.RequestID) == "" {
		snap.RequestID = draft.RequestID
	}
	if strings.TrimSpace(snap.SourceSystem) == "" {
		snap.SourceSystem = "farvoo"
	}
	if err := billsync.FreezeSourceLines(&snap); err != nil && len(snap.SourceLines) == 0 {
		return nil, err
	}

	payloadType := strings.TrimSpace(snap.ScopeType)

	existing, err := s.db.ListSignedSaleScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
	if err != nil {
		return nil, err
	}
	if err := checkScopeMutex(mode, existing); err != nil {
		return nil, err
	}

	// Already-issued scope → return original (NIF changes must not conflict).
	if hit := findIssuedScope(mode, strings.TrimSpace(in.ScopeID), draft.SourceSaleID, existing); hit != nil {
		rec, err := s.db.GetIssueRecordByID(hit.DocumentID)
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

	alloc, err := billsync.ParseAllocationJSON(draft.AllocationJSON)
	if err != nil {
		return nil, err
	}

	switch mode {
	case "whole_table":
		if payloadType != "whole_table" {
			return nil, coded("validation_failed", fmt.Sprintf("mode=whole_table requires payload whole_table (got %q)", payloadType))
		}
		if len(alloc.People) > 0 {
			return nil, coded("validation_failed", "cannot whole_table issue after local allocation; use person issue")
		}
	case "person":
		if in.AllocationRevision == nil {
			return nil, coded("validation_failed", "allocation_revision required for person issue")
		}
		if *in.AllocationRevision != draft.AllocationRevision {
			return nil, coded("allocation_conflict", "allocation was updated elsewhere; refresh and retry")
		}
		if len(alloc.People) == 0 {
			return nil, coded("validation_failed", "mode=person requires local allocation")
		}
	}

	var sale domain.SaleSnapshot
	var reqID string
	switch mode {
	case "whole_table":
		sale, err = billsync.DraftToSaleSnapshot(snap)
		reqID = "draft-issue:" + draft.ID + ":whole_table"
	case "person":
		sale, err = billsync.DraftPersonFromAllocation(snap, alloc, in.ScopeID)
		reqID = "draft-issue:" + draft.ID + ":person:" + strings.TrimSpace(in.ScopeID)
	}
	if err != nil {
		return nil, err
	}
	if err := billsync.ApplyCustomerOverride(&sale, in.CustomerNIF, in.CustomerName); err != nil {
		return nil, err
	}

	docType, err := ResolveSaleDocumentType(in.DocumentType)
	if err != nil {
		return nil, err
	}

	res, err := s.IssueDocument(ctx, domain.IssueRequest{
		StoreID: s.storeID, RequestID: reqID, OperatorID: operatorID, StationID: stationID, Snapshot: sale,
	}, docType)
	if err != nil {
		return nil, err
	}

	shouldDelete := mode == "whole_table"
	if mode == "person" {
		after, err := s.db.ListSignedSaleScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
		if err != nil {
			return nil, err
		}
		refs := make([]billsync.IssuedScopeRef, 0, len(after))
		for _, sc := range after {
			refs = append(refs, billsync.IssuedScopeRef{ScopeType: sc.ScopeType, ScopeID: sc.ScopeID})
		}
		peopleDone := billsync.AllAllocationPeopleIssued(alloc, refs)
		poolEmpty, err := billsync.PoolEmpty(snap, alloc)
		if err != nil {
			return nil, err
		}
		shouldDelete = peopleDone && poolEmpty
	}
	if shouldDelete {
		if err := s.db.DeleteBillDraftsBySale(draft.SourceSaleID); err != nil {
			res.CleanupPending = true
			return res, nil
		}
	}
	return res, nil
}

func checkScopeMutex(mode string, existing []store.SignedSaleScope) error {
	hasWhole, hasPerson := false, false
	for _, sc := range existing {
		switch strings.TrimSpace(sc.ScopeType) {
		case "whole_table":
			hasWhole = true
		case "person":
			hasPerson = true
		}
	}
	if mode == "person" && hasWhole {
		return coded("scope_mutex", "whole_table sale document already exists for this sale")
	}
	if mode == "whole_table" && hasPerson {
		return coded("scope_mutex", "person sale document already exists for this sale")
	}
	return nil
}

func findIssuedScope(mode, scopeID, saleID string, existing []store.SignedSaleScope) *store.SignedSaleScope {
	wantType := "whole_table"
	wantID := saleID
	if mode == "person" {
		wantType = "person"
		wantID = scopeID
	}
	for i := range existing {
		if strings.TrimSpace(existing[i].ScopeType) == wantType && strings.TrimSpace(existing[i].ScopeID) == wantID {
			return &existing[i]
		}
	}
	return nil
}

// DB exposes store for bill-sync puller wiring (read/ingest only via billsync package).
func (s *FiscalService) DB() *store.DB { return s.db }
