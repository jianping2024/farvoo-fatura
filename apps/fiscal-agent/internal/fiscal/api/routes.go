package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/uievents"
)

// HandlerDeps groups dependencies for HTTP handlers.
type HandlerDeps struct {
	Fiscal            *service.FiscalService
	StoreID           string
	DataDir           string
	Sessions          *SessionManager
	StationPrintersFn func() map[string]string // live Agent station_printers; may be nil
	StationMetaFn     func() []StationMeta     // cloud print_stations; may be nil
	UIEvents          *uievents.Hub            // Admin SSE; may be nil in unit tests
}

// Mount registers fiscal local routes. Prefix: /local/v1
func Mount(mux *http.ServeMux, deps HandlerDeps) {
	if deps.Sessions == nil {
		deps.Sessions = NewSessionManager(deps.DataDir)
	}
	registerFiscalRoutes(mux, deps)
}

func (deps HandlerDeps) guardAuto(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := routeAuthFor(r)
		if mode == authPublic {
			h(w, r)
			return
		}
		sess, err := deps.Sessions.ParseRequest(r)
		if err != nil || sess == nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
			return
		}
		ctx, ok := deps.sessionContext(w, r, sess)
		if !ok {
			return
		}
		sess = SessionFromContext(ctx)
		if !roleAllowed(mode, sess.Role) {
			switch mode {
			case authAdmin:
				forbiddenRole(w, "admin required")
			case authManager:
				forbiddenRole(w, "admin or owner required")
			default:
				forbiddenRole(w, "forbidden")
			}
			return
		}
		h(w, r.WithContext(ctx))
	}
}

func (deps HandlerDeps) guard(h http.HandlerFunc, minAuth routeAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if minAuth == authPublic {
			h(w, r)
			return
		}
		sess, err := deps.Sessions.ParseRequest(r)
		if err != nil || sess == nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
			return
		}
		ctx, ok := deps.sessionContext(w, r, sess)
		if !ok {
			return
		}
		sess = SessionFromContext(ctx)
		if !roleAllowed(minAuth, sess.Role) {
			switch minAuth {
			case authAdmin:
				forbiddenRole(w, "admin required")
			case authManager:
				forbiddenRole(w, "admin or owner required")
			default:
				forbiddenRole(w, "forbidden")
			}
			return
		}
		h(w, r.WithContext(ctx))
	}
}

func registerFiscalRoutes(mux *http.ServeMux, deps HandlerDeps) {
	g := deps.guardAuto
	mux.HandleFunc("GET /local/v1/health", g(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "module": "fiscal"})
	}))
	mux.HandleFunc("GET /local/v1/setup/status", g(func(w http.ResponseWriter, r *http.Request) {
		handleSetupStatus(w, r, deps)
	}))
	mux.HandleFunc("PUT /local/v1/setup/taxpayer", g(func(w http.ResponseWriter, r *http.Request) {
		handleUpsertTaxpayer(w, r, deps)
	}))
	mux.HandleFunc("PUT /local/v1/setup/at-credentials", g(func(w http.ResponseWriter, r *http.Request) {
		handleUpsertAT(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/series/register", g(func(w http.ResponseWriter, r *http.Request) {
		handleRegisterSeries(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/activate", g(func(w http.ResponseWriter, r *http.Request) {
		handleActivate(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/activate-from-cloud", g(func(w http.ResponseWriter, r *http.Request) {
		handleActivateFromCloud(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/setup/operators", g(func(w http.ResponseWriter, r *http.Request) {
		handleListOperators(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/setup/operators/manage", g(func(w http.ResponseWriter, r *http.Request) {
		handleListOperatorsManage(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/bootstrap-owner", g(func(w http.ResponseWriter, r *http.Request) {
		handleBootstrapOwner(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/login", g(func(w http.ResponseWriter, r *http.Request) {
		handleLogin(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/logout", g(func(w http.ResponseWriter, r *http.Request) {
		handleLogout(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/change-pin", g(func(w http.ResponseWriter, r *http.Request) {
		handleChangePIN(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/terminals/pair", g(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalPair(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/setup/terminals/summary", g(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalSummary(w, r, deps)
	}))
	mux.HandleFunc("PUT /local/v1/setup/operator", g(func(w http.ResponseWriter, r *http.Request) {
		handleOperator(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/backup", g(func(w http.ResponseWriter, r *http.Request) {
		handleBackupFiscalDB(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/integrity/verify", g(func(w http.ResponseWriter, r *http.Request) {
		handleVerifyIntegrity(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/setup/prepare-swap", g(func(w http.ResponseWriter, r *http.Request) {
		handlePrepareSwap(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/fiscal-documents", g(func(w http.ResponseWriter, r *http.Request) {
		handleIssue(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/fiscal-documents/manual", g(func(w http.ResponseWriter, r *http.Request) {
		handleIssueManualFT(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/products", g(func(w http.ResponseWriter, r *http.Request) {
		handleListProducts(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/products", g(func(w http.ResponseWriter, r *http.Request) {
		handleUpsertProduct(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/customers", g(func(w http.ResponseWriter, r *http.Request) {
		handleListCustomers(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/customers", g(func(w http.ResponseWriter, r *http.Request) {
		handleUpsertCustomer(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/fiscal-documents", g(func(w http.ResponseWriter, r *http.Request) {
		handleListFiscalDocuments(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/fiscal-documents/{documentId}", g(func(w http.ResponseWriter, r *http.Request) {
		handleGetFiscalDocument(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/fiscal-documents/{documentId}/reprints", g(func(w http.ResponseWriter, r *http.Request) {
		handleReprint(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/fiscal-documents/{documentId}/credit-notes", g(func(w http.ResponseWriter, r *http.Request) {
		handleCreditNote(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/fiscal-documents/{documentId}/debit-notes", g(func(w http.ResponseWriter, r *http.Request) {
		handleDebitNote(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/saft/exports", g(func(w http.ResponseWriter, r *http.Request) {
		handleExportSAFT(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/saft/exports", g(func(w http.ResponseWriter, r *http.Request) {
		handleListSAFTExports(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/saft/exports/{exportId}", g(func(w http.ResponseWriter, r *http.Request) {
		handleGetSAFTExport(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/saft/exports/{exportId}/download", g(func(w http.ResponseWriter, r *http.Request) {
		handleDownloadSAFTExport(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/fiscal-documents/by-request/{requestId}", g(func(w http.ResponseWriter, r *http.Request) {
		handleGetByRequest(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/print-jobs/{printJobId}", g(func(w http.ResponseWriter, r *http.Request) {
		handleGetPrintJob(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/printers", g(func(w http.ResponseWriter, r *http.Request) {
		handleListPrinters(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/bill-drafts", g(func(w http.ResponseWriter, r *http.Request) {
		handleListBillDrafts(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/bill-drafts/{id}", g(func(w http.ResponseWriter, r *http.Request) {
		handleGetBillDraft(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/bill-drafts/{id}/issue", g(func(w http.ResponseWriter, r *http.Request) {
		handleIssueBillDraft(w, r, deps)
	}))
	mux.HandleFunc("PUT /local/v1/bill-drafts/{id}/allocation", g(func(w http.ResponseWriter, r *http.Request) {
		handleSaveBillDraftAllocation(w, r, deps)
	}))
	mux.HandleFunc("POST /local/v1/bill-drafts/{id}/discard", g(func(w http.ResponseWriter, r *http.Request) {
		handleDiscardBillDraft(w, r, deps)
	}))
	mux.HandleFunc("GET /local/v1/events", func(w http.ResponseWriter, r *http.Request) {
		if deps.UIEvents == nil {
			writeErr(w, http.StatusServiceUnavailable, "events_unavailable", "UI events hub not configured")
			return
		}
		deps.UIEvents.ServeSSE(w, r)
	})
	// Dev/UAT only: run the SAME PullAndIngest path in-process so SSE Hub sees draft writers.
	mux.HandleFunc("POST /local/v1/dev/bill-sync/pull", func(w http.ResponseWriter, r *http.Request) {
		handleDevBillSyncPull(w, r, deps)
	})
}

// handleDevBillSyncPull is the ONLY HTTP trigger for billsync.PullAndIngest (fiscal-local UAT).
// Gated by FISCAL_ALLOW_DEV_KEY=1. Uses FARVOO_API + FARVOO_JWT — same as production doorbell.
func handleDevBillSyncPull(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if os.Getenv("FISCAL_ALLOW_DEV_KEY") != "1" {
		writeErr(w, http.StatusForbidden, "dev_forbidden", "set FISCAL_ALLOW_DEV_KEY=1 for local UAT pull")
		return
	}
	if deps.Fiscal == nil || deps.Fiscal.DB() == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	apiBase := strings.TrimSpace(os.Getenv("FARVOO_API"))
	jwt := strings.TrimSpace(os.Getenv("FARVOO_JWT"))
	if apiBase == "" || jwt == "" {
		writeErr(w, http.StatusBadRequest, "farvoo_env_missing", "FARVOO_API and FARVOO_JWT required")
		return
	}
	n, err := (&billsync.Puller{APIBase: apiBase, JWT: jwt, DB: deps.Fiscal.DB()}).PullAndIngest(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "bill_sync_pull_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"processed": n})
}

func handleListPrinters(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	mapped := map[string]string{}
	if deps.StationPrintersFn != nil {
		mapped = deps.StationPrintersFn()
	}
	var meta []StationMeta
	if deps.StationMetaFn != nil {
		meta = deps.StationMetaFn()
	}
	stations := BuildPrinterStationList(mapped, meta)
	if stations == nil {
		stations = []PrinterStation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"stations": stations})
}

func handleSetupStatus(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	st, err := deps.Fiscal.SetupStatus(r.URL.Query().Get("store_id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "status_failed", err.Error())
		return
	}
	if s := SessionFromContext(r.Context()); s != nil && deps.Fiscal.DB() != nil {
		if can, err := deps.Fiscal.DB().OperatorCanIssueNC(deps.StoreID, s.OperatorID); err == nil {
			st.OperatorCanIssueNC = can
			st.ReadyToCredit = st.NCSeriesOK && st.ActivatedOK && st.OperatorOK && can
			st.ReadyToDebit = st.NDSeriesOK && st.ActivatedOK && st.OperatorOK && can
		}
	} else if deps.Sessions != nil && deps.Fiscal.DB() != nil {
		if sess, err := deps.Sessions.ParseRequest(r); err == nil && sess != nil {
			if can, err := deps.Fiscal.DB().OperatorCanIssueNC(deps.StoreID, sess.OperatorID); err == nil {
				st.OperatorCanIssueNC = can
				st.ReadyToCredit = st.NCSeriesOK && st.ActivatedOK && st.OperatorOK && can
				st.ReadyToDebit = st.NDSeriesOK && st.ActivatedOK && st.OperatorOK && can
			}
		}
	}
	writeJSON(w, http.StatusOK, st)
}

func handleBackupFiscalDB(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	path, size, err := deps.Fiscal.BackupFiscalDB()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "backup_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup_path": path, "bytes": size})
}

func handleVerifyIntegrity(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		OperatorID  string `json:"operator_id"`
		BlockOnFail *bool  `json:"block_on_fail"`
		HealOnPass  bool   `json:"heal_on_pass"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	block := true
	if body.BlockOnFail != nil {
		block = *body.BlockOnFail
	}
	id, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	body.OperatorID = id
	rep, err := deps.Fiscal.VerifySeriesIntegrity(block, body.HealOnPass, body.OperatorID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "integrity_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func handlePrepareSwap(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		OperatorID string `json:"operator_id"`
		Backup     *bool  `json:"backup"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	doBackup := true
	if body.Backup != nil {
		doBackup = *body.Backup
	}
	id, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	body.OperatorID = id
	path, size, err := deps.Fiscal.PrepareMachineSwap(doBackup, body.OperatorID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "prepare_swap_failed", err.Error())
		return
	}
	st, _ := deps.Fiscal.SetupStatus(deps.StoreID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"backup_path":   path,
		"backup_bytes":  size,
		"activated_ok":  st != nil && st.ActivatedOK,
		"ready_to_issue": st != nil && st.ReadyToIssue,
		"next_steps": []string{
			"Copy backup_path to the new PC",
			"Stop Agent; replace fiscal.db with the backup (remove -wal/-shm if present)",
			"Start Agent; POST /local/v1/setup/integrity/verify",
			"POST /local/v1/setup/activate-from-cloud (or local PEM in UAT)",
		},
	})
}

func handleUpsertTaxpayer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body store.TaxpayerInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	if err := deps.Fiscal.UpsertTaxpayer(body); err != nil {
		writeCoded(w, err)
		return
	}
	st, _ := deps.Fiscal.SetupStatus(body.StoreID)
	writeJSON(w, http.StatusOK, st)
}

func handleUpsertAT(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID  string `json:"store_id"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	if err := deps.Fiscal.UpsertATCredentials(body.StoreID, body.Username, body.Password); err != nil {
		writeCoded(w, err)
		return
	}
	st, _ := deps.Fiscal.SetupStatus(body.StoreID)
	writeJSON(w, http.StatusOK, st)
}

func handleRegisterSeries(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID    string `json:"store_id"`
		SeriesCode string `json:"series_code"`
		DocType    string `json:"document_type"`
		FiscalYear int    `json:"fiscal_year"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	res, err := deps.Fiscal.RegisterSeries(r.Context(), body.StoreID, body.SeriesCode, body.DocType, body.FiscalYear)
	if err != nil {
		writeCoded(w, err)
		return
	}
	out := map[string]any{
		"idempotent_hit": res.IdempotentHit,
		"series_code":    res.SeriesCode,
		"document_type":  res.DocumentType,
	}
	if res.Status != nil {
		b, _ := json.Marshal(res.Status)
		_ = json.Unmarshal(b, &out)
		out["idempotent_hit"] = res.IdempotentHit
		out["series_code"] = res.SeriesCode
		out["document_type"] = res.DocumentType
	}
	writeJSON(w, http.StatusOK, out)
}

func handleActivate(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID              string `json:"store_id"`
		ProductPrivateKeyPEM string `json:"product_private_key_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	st, err := deps.Fiscal.ActivateFiscal(body.StoreID, body.ProductPrivateKeyPEM)
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func handleActivateFromCloud(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID string `json:"store_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	st, err := deps.Fiscal.ActivateFromCloud(r.Context(), body.StoreID)
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func handleListOperatorsManage(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	db := deps.Fiscal.DB()
	if db == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "db missing")
		return
	}
	rows, err := db.ListOperatorsForManage(deps.StoreID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if rows == nil {
		rows = []store.OperatorManageRow{}
	}
	if s := SessionFromContext(r.Context()); s != nil && s.Role == "owner" {
		filtered := make([]store.OperatorManageRow, 0, len(rows))
		for _, row := range rows {
			if row.Role == "cashier" {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"operators": rows})
}

func setSessionCookieFromState(w http.ResponseWriter, deps HandlerDeps, operatorID string) {
	sm := deps.Sessions
	if sm == nil {
		sm = NewSessionManager(deps.DataDir)
	}
	st, err := deps.Fiscal.DB().GetOperatorSessionState(deps.StoreID, operatorID)
	if err != nil || st == nil || !st.Active {
		return
	}
	_ = sm.SetSessionCookie(w, Session{
		OperatorID:  operatorID,
		Role:        st.Role,
		DisplayName: st.DisplayName,
		Epoch:       st.SessionEpoch,
	})
}

func handleOperator(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		ID          string `json:"id"`
		StoreID     string `json:"store_id"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
		PIN         string `json:"pin"`
		Active      *bool  `json:"active"`
		CanIssueNC  *bool  `json:"can_issue_nc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	if body.ID == "" {
		writeErr(w, http.StatusBadRequest, "id_required", "operator id required")
		return
	}
	actorRole := ""
	actorID := ""
	if s := SessionFromContext(r.Context()); s != nil {
		actorRole = s.Role
		actorID = s.OperatorID
	}
	touched := false
	if body.DisplayName != "" || body.Role != "" {
		touched = true
		if body.DisplayName == "" {
			writeErr(w, http.StatusBadRequest, "display_name_required", "display_name required")
			return
		}
		if err := deps.Fiscal.UpsertOperatorWithActor(actorRole, actorID, body.ID, body.StoreID, body.Role, body.DisplayName); err != nil {
			writeCoded(w, err)
			return
		}
	}
	if body.Active != nil {
		touched = true
		if err := deps.Fiscal.SetOperatorActiveWithActor(actorRole, body.StoreID, body.ID, *body.Active); err != nil {
			writeCoded(w, err)
			return
		}
		action := "OPERATOR_ACTIVATE"
		if !*body.Active {
			action = "OPERATOR_DEACTIVATE"
		}
		_ = deps.Fiscal.DB().InsertAuditLog(actorID, action, "operator", body.ID, "{}")
	}
	if body.PIN != "" {
		touched = true
		if err := deps.Fiscal.SetOperatorPINWithActor(actorRole, actorID, body.StoreID, body.ID, body.PIN); err != nil {
			writeCoded(w, err)
			return
		}
		_ = deps.Fiscal.DB().InsertAuditLog(actorID, "PIN_RESET", "operator", body.ID, "{}")
	}
	if body.CanIssueNC != nil {
		touched = true
		if err := deps.Fiscal.SetOperatorCanIssueNCWithActor(actorRole, body.StoreID, body.ID, *body.CanIssueNC); err != nil {
			writeCoded(w, err)
			return
		}
	}
	if !touched {
		writeErr(w, http.StatusBadRequest, "no_updates", "no operator fields to update")
		return
	}
	st, _ := deps.Fiscal.SetupStatus(body.StoreID)
	writeJSON(w, http.StatusOK, st)
}

type issueBody struct {
	StoreID    string              `json:"store_id"`
	RequestID  string              `json:"request_id"`
	OperatorID string              `json:"operator_id"`
	StationID  string              `json:"station_id"`
	DocType    string              `json:"document_type"`
	Snapshot   domain.SaleSnapshot `json:"snapshot"`
}

func handleIssue(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body issueBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	id, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	body.OperatorID = id
	docType := domain.DocumentType(body.DocType)
	if docType == "" {
		docType = domain.DefaultSaleDocumentType
	}
	res, err := deps.Fiscal.IssueDocument(r.Context(), domain.IssueRequest{
		StoreID: body.StoreID, RequestID: body.RequestID, OperatorID: body.OperatorID,
		StationID: body.StationID, Snapshot: body.Snapshot,
	}, docType)
	if errors.Is(err, store.ErrConflict) {
		writeErr(w, http.StatusConflict, "idempotency_conflict", err.Error())
		return
	}
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id":     res.DocumentID,
		"invoice_no":      res.InvoiceNo,
		"atcud":           res.ATCUD,
		"document_type":   res.DocumentType,
		"document_status": res.DocumentStatus,
		"print_job_id":    res.PrintJobID,
		"print_status":    res.PrintStatus,
		"issued_at":       res.IssuedAt.UTC().Format(time.RFC3339),
		"idempotent_hit":  res.IdempotentHit,
	})
}

func handleGetByRequest(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	requestID := r.PathValue("requestId")
	storeID := r.URL.Query().Get("store_id")
	if storeID == "" {
		storeID = deps.StoreID
	}
	res, err := deps.Fiscal.GetByRequestID(storeID, requestID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id": res.DocumentID, "invoice_no": res.InvoiceNo, "atcud": res.ATCUD,
		"document_type": res.DocumentType, "document_status": res.DocumentStatus,
		"print_job_id": res.PrintJobID, "print_status": res.PrintStatus,
		"issued_at": res.IssuedAt.UTC().Format(time.RFC3339), "idempotent_hit": true,
	})
}

func handleGetPrintJob(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	job, err := deps.Fiscal.GetPrintJob(r.PathValue("printJobId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "print job not found")
		return
	}
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func handleListBillDrafts(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	list, err := deps.Fiscal.ListBillDrafts(50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	type row struct {
		ID               string `json:"id"`
		RequestID        string `json:"request_id"`
		SourceSaleID     string `json:"source_sale_id"`
		Status           string `json:"status"`
		CloudJobID       string `json:"cloud_job_id,omitempty"`
		UpdatedAt        string `json:"updated_at"`
		TableDisplayName string `json:"table_display_name,omitempty"`
		ScopeType        string `json:"scope_type,omitempty"`
		GrossTotal       string `json:"gross_total,omitempty"`
	}
	out := make([]row, 0, len(list))
	for _, d := range list {
		var meta struct {
			TableDisplayName string `json:"table_display_name"`
			ScopeType        string `json:"scope_type"`
		}
		_ = json.Unmarshal([]byte(d.PayloadJSON), &meta)
		out = append(out, row{
			ID: d.ID, RequestID: d.RequestID, SourceSaleID: d.SourceSaleID,
			Status: d.Status, CloudJobID: d.CloudJobID, UpdatedAt: d.UpdatedAt,
			TableDisplayName: meta.TableDisplayName, ScopeType: meta.ScopeType,
			GrossTotal:       billsync.ListGrossTotalFromPayload(d.PayloadJSON),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": out})
}

func handleGetBillDraft(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	detail, err := deps.Fiscal.GetBillDraftDetail(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func handleDiscardBillDraft(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	if err := deps.Fiscal.DiscardBillDrafts(r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "discard_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleIssueBillDraft(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	draftID := r.PathValue("id")
	var body struct {
		OperatorID         string `json:"operator_id"`
		DocumentType       string `json:"document_type"`
		Mode               string `json:"mode"`
		ScopeID              string `json:"scope_id"`
		StationID            string `json:"station_id"`
		CustomerNIF          string `json:"customer_nif"`
		CustomerName         string `json:"customer_name"`
		AllocationRevision   *int64 `json:"allocation_revision"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	opID, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	body.OperatorID = opID
	if body.Mode == "" {
		body.Mode = "whole_table"
	}
	res, err := deps.Fiscal.IssueFromBillDraft(r.Context(), service.IssueBillDraftInput{
		DraftID: draftID, DocumentType: body.DocumentType, OperatorID: body.OperatorID, Mode: body.Mode, ScopeID: body.ScopeID,
		StationID: body.StationID, CustomerNIF: body.CustomerNIF, CustomerName: body.CustomerName,
		AllocationRevision: body.AllocationRevision,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		if ie := billsync.AsIngestError(err); ie != nil {
			writeErr(w, http.StatusBadRequest, ie.Code, ie.Message)
			return
		}
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func handleSaveBillDraftAllocation(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		ExpectedRevision int64               `json:"expected_revision"`
		Allocation       billsync.Allocation `json:"allocation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_failed", "invalid json")
		return
	}
	detail, err := deps.Fiscal.SaveBillDraftAllocation(service.SaveBillDraftAllocationInput{
		DraftID: r.PathValue("id"), ExpectedRevision: body.ExpectedRevision, Allocation: body.Allocation,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		if ie := billsync.AsIngestError(err); ie != nil {
			writeErr(w, http.StatusBadRequest, ie.Code, ie.Message)
			return
		}
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func writeCoded(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrLastOwnerConstraint) {
		writeErr(w, http.StatusConflict, "last_owner_constraint", err.Error())
		return
	}
	if errors.Is(err, store.ErrLastAdminConstraint) {
		writeErr(w, http.StatusConflict, "last_admin_constraint", err.Error())
		return
	}
	var ce *service.CodedError
	if errors.As(err, &ce) {
		status := http.StatusBadRequest
		switch ce.Code {
		case service.ErrCodeForbidden:
			status = http.StatusForbidden
		case service.ErrCodeATSOAPFailed:
			status = http.StatusBadGateway
		case service.ErrCodeSignerNotReady, service.ErrCodeOpsActivatePending, service.ErrCodeFiscalProfileMissing, service.ErrCodeSeriesMissing, service.ErrCodeTaxpayerMissing, service.ErrCodeATCredsMissing,
			service.ErrCodeSeriesAlreadyActive,
			service.ErrCodeCreditNotAllowed, service.ErrCodeCreditAmountExceeded, service.ErrCodeIdempotencyConflict,
			service.ErrCodeNoInvoices, service.ErrCodeReprintNotAllowed:
			status = http.StatusConflict
		case "scope_mutex", "allocation_conflict", "draft_not_open", "already_invoiced":
			status = http.StatusConflict
		case "validation_failed":
			status = http.StatusBadRequest
		}
		writeErr(w, status, ce.Code, ce.Msg)
		return
	}
	writeErr(w, http.StatusBadRequest, "issue_failed", err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": code, "message": msg})
}
