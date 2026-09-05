package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

type testEnv struct {
	DB        *store.DB
	Sig       *signer.PEMSigner
	StatePath string
	DBPath    string
	State     *ToolState
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal.db")
	statePath := filepath.Join(dir, StateFileName)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	keyPath := filepath.Join("..", "..", "internal", "fiscal", "testdata", "dev_signing_key.pem")
	sig, err := signer.LoadPEMFile(keyPath, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := sig.PublicKeyPEM()
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "FS", "FS2026DEMO01", "FSVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	st, err := LoadToolState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{DB: db, Sig: sig, StatePath: statePath, DBPath: dbPath, State: st}
}

func (e *testEnv) planIn(date, keep string) SettlePlanInput {
	return SettlePlanInput{BusinessDate: date, KeepTarget: keep, DBPath: e.DBPath, State: e.State}
}

func (e *testEnv) apply(t *testing.T, plan *SettlePlan) *SettleResult {
	t.Helper()
	res, err := ApplySettlement(context.Background(), e.DB.SQL, e.StatePath, e.State, e.DBPath, plan)
	if err != nil {
		t.Fatal(err)
	}
	// reload state after save
	st, err := LoadToolState(e.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	e.State = st
	return res
}

func issueFS(t *testing.T, db *store.DB, sig *signer.PEMSigner, reqID, gross, pay, taxID string, day time.Time) string {
	t.Helper()
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: reqID, DocType: domain.DocumentFS,
		OperatorID: "op-demo-cashier", NowUTC: day,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: reqID, ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: gross, VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: taxID, CompanyName: "Cliente", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: pay, Amount: gross}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rec.DocumentID
}

func bizDay() time.Time {
	return time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
}

func ensureCorrectiveSeries(t *testing.T, db *store.DB) {
	t.Helper()
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "ND", "ND2026DEMO01", "NDVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
}

func TestAuth_InitialPINForceChange(t *testing.T) {
	e := newTestEnv(t)
	st, err := LoginPIN(e.StatePath, InitialDevtoolPIN)
	if err != nil || !st.MustChangePIN {
		t.Fatalf("first login: err=%v mustChange=%v", err, st)
	}
	if _, err := LoginPIN(e.StatePath, "222222"); err == nil {
		t.Fatal("wrong pin must fail")
	}
	if err := ChangePIN(e.StatePath, InitialDevtoolPIN, "654321"); err != nil {
		t.Fatal(err)
	}
	st2, err := LoginPIN(e.StatePath, "654321")
	if err != nil || st2.MustChangePIN {
		t.Fatalf("after change: err=%v must=%v", err, st2)
	}
	if _, err := LoginPIN(e.StatePath, InitialDevtoolPIN); err == nil {
		t.Fatal("old pin must fail")
	}
}

func TestIsProtectedInvoice(t *testing.T) {
	if IsProtectedInvoice("CASH", "999999990", false) {
		t.Fatal("cash+final must not protect")
	}
	if !IsProtectedInvoice("CARD", "999999990", false) {
		t.Fatal("card must protect")
	}
	if !IsProtectedInvoice("CASH", "123456789", false) {
		t.Fatal("real NIF must protect")
	}
	if !IsProtectedInvoice("CASH", "999999990", true) {
		t.Fatal("NC/ND original must protect")
	}
}

func TestState_NoDevtoolTablesInFiscalDB(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "t1", "10.00", "CASH", "999999990", day)
	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "10"))
	if err != nil {
		t.Fatal(err)
	}
	e.apply(t, plan)
	var n int
	err = e.DB.SQL.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name LIKE '_devtool%'`).Scan(&n)
	if err != nil || n != 0 {
		t.Fatalf("fiscal.db must not have _devtool_* tables, n=%d err=%v", n, err)
	}
	if !IsDateSettledInState(e.State, mustNorm(t, e.DBPath), "2026-08-20") {
		t.Fatal("settlement must be in state file")
	}
}

func mustNorm(t *testing.T, p string) string {
	t.Helper()
	n, err := NormalizeDBPath(p)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSettle_NoProtected_KeepAndDelete(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "a", "40.00", "CASH", "999999990", day)
	issueFS(t, e.DB, e.Sig, "b", "40.00", "CASH", "999999990", day.Add(time.Minute))
	issueFS(t, e.DB, e.Sig, "c", "30.00", "CASH", "999999990", day.Add(2*time.Minute))
	issueFS(t, e.DB, e.Sig, "d", "20.00", "CASH", "999999990", day.Add(3*time.Minute))

	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.KeepActual != "110.00" || plan.Shortfall || len(plan.DeleteIDs) != 1 {
		t.Fatalf("%+v", plan)
	}
	res := e.apply(t, plan)
	if res.DeletedCount != 1 {
		t.Fatal(res)
	}
	var n int
	_ = e.DB.SQL.QueryRow(`SELECT COUNT(1) FROM invoices WHERE document_type='FS'`).Scan(&n)
	if n != 3 {
		t.Fatalf("remaining FS=%d", n)
	}
	rep, err := e.DB.VerifySeriesIntegrity(store.VerifySeriesIntegrityOptions{})
	if err != nil || !rep.OK {
		t.Fatal(rep, err)
	}
	_, err = BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "10"))
	if err == nil || !strings.Contains(err.Error(), "已结算") {
		t.Fatalf("want settled error, got %v", err)
	}
}

func TestSettle_ProtectedAnchor_CountOnlyAfter(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "p1", "40.00", "CASH", "999999990", day)
	issueFS(t, e.DB, e.Sig, "p2", "40.00", "CASH", "999999990", day.Add(time.Minute))
	issueFS(t, e.DB, e.Sig, "card", "30.00", "CARD", "999999990", day.Add(2*time.Minute))
	issueFS(t, e.DB, e.Sig, "p4", "50.00", "CASH", "999999990", day.Add(3*time.Minute))
	issueFS(t, e.DB, e.Sig, "p5", "60.00", "CASH", "999999990", day.Add(4*time.Minute))
	issueFS(t, e.DB, e.Sig, "p6", "10.00", "CASH", "999999990", day.Add(5*time.Minute))

	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.KeepActual != "110.00" || len(plan.DeleteIDs) != 1 || plan.DeleteGrossTotal != "10.00" {
		t.Fatalf("%+v", plan)
	}
	e.apply(t, plan)
}

func TestSettle_RealNIFProtected(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "c1", "80.00", "CASH", "999999990", day)
	issueFS(t, e.DB, e.Sig, "nif", "5.00", "CASH", "123456789", day.Add(time.Minute))
	issueFS(t, e.DB, e.Sig, "c2", "100.00", "CASH", "999999990", day.Add(2*time.Minute))
	issueFS(t, e.DB, e.Sig, "c3", "1.00", "CASH", "999999990", day.Add(3*time.Minute))

	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.KeepActual != "100.00" || len(plan.DeleteIDs) != 1 {
		t.Fatalf("%+v", plan)
	}
}

func TestSettle_Shortfall_ZeroDeleteAllowed(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "s1", "40.00", "CASH", "999999990", day)
	issueFS(t, e.DB, e.Sig, "s2", "40.00", "CASH", "999999990", day.Add(time.Minute))

	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Shortfall || len(plan.DeleteIDs) != 0 {
		t.Fatalf("%+v", plan)
	}
	res := e.apply(t, plan)
	if res.DeletedCount != 0 {
		t.Fatal(res)
	}
	var fs int
	_ = e.DB.SQL.QueryRow(`SELECT COUNT(1) FROM invoices WHERE document_type='FS'`).Scan(&fs)
	if fs != 2 {
		t.Fatalf("fs=%d", fs)
	}
}

func TestSettle_ProtectedIsLast_ZeroDelete(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "x1", "50.00", "CASH", "999999990", day)
	issueFS(t, e.DB, e.Sig, "x2", "50.00", "CARD", "999999990", day.Add(time.Minute))
	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Shortfall || len(plan.DeleteIDs) != 0 {
		t.Fatalf("%+v", plan)
	}
	e.apply(t, plan)
}

func TestSettle_NCOriginalAsProtectedAnchor(t *testing.T) {
	e := newTestEnv(t)
	ensureCorrectiveSeries(t, e.DB)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "n1", "40.00", "CASH", "999999990", day)
	issueFS(t, e.DB, e.Sig, "n2", "40.00", "CASH", "999999990", day.Add(time.Minute))
	orig := issueFS(t, e.DB, e.Sig, "n3", "30.00", "CASH", "999999990", day.Add(2*time.Minute))
	issueFS(t, e.DB, e.Sig, "n4", "50.00", "CASH", "999999990", day.Add(3*time.Minute))
	issueFS(t, e.DB, e.Sig, "n5", "60.00", "CASH", "999999990", day.Add(4*time.Minute))
	issueFS(t, e.DB, e.Sig, "n6", "10.00", "CASH", "999999990", day.Add(5*time.Minute))
	if _, err := e.DB.IssueNC(context.Background(), e.Sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-on-n3", OriginalInvoiceID: orig,
		OperatorID: "op-demo-cashier", Reason: "teste NC", CreditFull: true,
		NowUTC: day.Add(6 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.KeepActual != "110.00" || plan.AnchorSeq != 3 || len(plan.DeleteIDs) != 1 {
		t.Fatalf("%+v", plan)
	}
	e.apply(t, plan)
}

func TestSettle_NDOriginalAsProtectedAnchor(t *testing.T) {
	e := newTestEnv(t)
	ensureCorrectiveSeries(t, e.DB)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "dpre", "20.00", "CASH", "999999990", day)
	orig := issueFS(t, e.DB, e.Sig, "dnd", "15.00", "CASH", "999999990", day.Add(time.Minute))
	issueFS(t, e.DB, e.Sig, "dpost", "100.00", "CASH", "999999990", day.Add(2*time.Minute))
	issueFS(t, e.DB, e.Sig, "dtail", "5.00", "CASH", "999999990", day.Add(3*time.Minute))
	if _, err := e.DB.IssueND(context.Background(), e.Sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-on-fs", OriginalInvoiceID: orig,
		OperatorID: "op-demo-cashier", Reason: "teste ND", DebitFull: true,
		NowUTC: day.Add(4 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.KeepActual != "100.00" || len(plan.DeleteIDs) != 1 {
		t.Fatalf("%+v", plan)
	}
	e.apply(t, plan)
}

func TestSettle_NCOriginalIsLast_ZeroDelete(t *testing.T) {
	e := newTestEnv(t)
	ensureCorrectiveSeries(t, e.DB)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "z1", "80.00", "CASH", "999999990", day)
	orig := issueFS(t, e.DB, e.Sig, "z2", "20.00", "CASH", "999999990", day.Add(time.Minute))
	if _, err := e.DB.IssueNC(context.Background(), e.Sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-last", OriginalInvoiceID: orig,
		OperatorID: "op-demo-cashier", Reason: "last NC", CreditFull: true,
		NowUTC: day.Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "100"))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Shortfall || len(plan.DeleteIDs) != 0 {
		t.Fatalf("%+v", plan)
	}
	e.apply(t, plan)
}

func TestSettle_RejectNonTipDay(t *testing.T) {
	e := newTestEnv(t)
	d1 := bizDay()
	d2 := time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC)
	issueFS(t, e.DB, e.Sig, "d1a", "10.00", "CASH", "999999990", d1)
	issueFS(t, e.DB, e.Sig, "d2a", "10.00", "CASH", "999999990", d2)
	_, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-20", "10"))
	if err == nil || !strings.Contains(err.Error(), "tip") {
		t.Fatalf("want tip reject, got %v", err)
	}
	plan, err := BuildSettlePlan(context.Background(), e.DB.SQL, e.planIn("2026-08-21", "10"))
	if err != nil {
		t.Fatal(err)
	}
	_ = plan
}

func TestSettle_ValidateEligibleDates(t *testing.T) {
	e := newTestEnv(t)
	day := bizDay()
	issueFS(t, e.DB, e.Sig, "e1", "10.00", "CASH", "999999990", day)
	tz, err := LoadTaxpayerTimezone(e.DB.SQL)
	if err != nil {
		t.Fatal(err)
	}
	el, err := ListEligibleSettleDates(e.DB.SQL, e.State, e.DBPath, tz)
	if err != nil || len(el) != 1 || el[0] != "2026-08-20" {
		t.Fatalf("eligible=%v err=%v", el, err)
	}
	if err := ValidateSettleDate("2026-08-19", el); err == nil {
		t.Fatal("bad date must fail")
	}
}

func TestSettle_StateKeyedByDBPath(t *testing.T) {
	e1 := newTestEnv(t)
	day := bizDay()
	issueFS(t, e1.DB, e1.Sig, "k1", "10.00", "CASH", "999999990", day)
	plan, err := BuildSettlePlan(context.Background(), e1.DB.SQL, e1.planIn("2026-08-20", "10"))
	if err != nil {
		t.Fatal(err)
	}
	e1.apply(t, plan)

	// same state file, different db path → same calendar day still eligible for other db
	e2dir := t.TempDir()
	db2path := filepath.Join(e2dir, "other.db")
	db2, err := store.Open(db2path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	keyPath := filepath.Join("..", "..", "internal", "fiscal", "testdata", "dev_signing_key.pem")
	sig := e1.Sig
	pub, _ := sig.PublicKeyPEM()
	if err := db2.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}
	if err := db2.UpsertActiveSeries("store-demo-001", "FS", "FS2026DEMO01", "FSVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	issueFS(t, db2, sig, "o1", "10.00", "CASH", "999999990", day)
	st, err := LoadToolState(e1.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := BuildSettlePlan(context.Background(), db2.SQL, SettlePlanInput{
		BusinessDate: "2026-08-20", KeepTarget: "10", DBPath: db2path, State: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySettlement(context.Background(), db2.SQL, e1.StatePath, st, db2path, plan2); err != nil {
		t.Fatal(err)
	}
}

func TestSettle_UniqueWritersPresent(t *testing.T) {
	_ = ApplySettlement
	_ = BuildSettlePlan
	_ = LoginPIN
	_ = ChangePIN
	_ = LoadToolState
	_ = SaveToolState
	_ = MarkSettledInState
	_ = IsProtectedInvoice
	_ = IsDateSettledInState
}
