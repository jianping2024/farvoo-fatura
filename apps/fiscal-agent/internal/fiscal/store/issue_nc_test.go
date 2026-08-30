package store_test

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

func seedDemoWithNC(t *testing.T, db *store.DB, sig *signer.PEMSigner) (ftDocID string) {
	t.Helper()
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
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
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-req-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", NowUTC: now,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-1", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "12.50", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "12.50"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rec.DocumentID
}

func TestIssueNCFullCredit(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	ftID := seedDemoWithNC(t, db, sig)

	nc, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-req-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Devolucao total", CreditFull: true,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if nc.InvoiceNo != "NC NC2026DEMO01/1" {
		t.Fatalf("invoice_no %s", nc.InvoiceNo)
	}
	if nc.DocumentType != domain.DocumentNC {
		t.Fatalf("type %s", nc.DocumentType)
	}

	var ftStatus, credited string
	if err := db.SQL.QueryRow(`SELECT document_status, credited_gross_total FROM invoices WHERE id=?`, ftID).
		Scan(&ftStatus, &credited); err != nil {
		t.Fatal(err)
	}
	if ftStatus != string(domain.DocumentCreditedFull) || credited != "12.50" {
		t.Fatalf("original status=%s credited=%s", ftStatus, credited)
	}

	var ftSeq, ncSeq int64
	_ = db.SQL.QueryRow(`SELECT last_number FROM series WHERE series_code='FT2026DEMO01'`).Scan(&ftSeq)
	_ = db.SQL.QueryRow(`SELECT last_number FROM series WHERE series_code='NC2026DEMO01'`).Scan(&ncSeq)
	if ftSeq != 1 || ncSeq != 1 {
		t.Fatalf("series seq ft=%d nc=%d", ftSeq, ncSeq)
	}

	var origNo, reason string
	err = db.SQL.QueryRow(`SELECT original_invoice_no, reason FROM invoice_line_references LIMIT 1`).Scan(&origNo, &reason)
	if err != nil || origNo != "FT FT2026DEMO01/1" || reason != "Devolucao total" {
		t.Fatalf("ref orig=%s reason=%s err=%v", origNo, reason, err)
	}
}

func TestIssueNCIdempotency(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	ftID := seedDemoWithNC(t, db, sig)
	p := store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-idem-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Test", CreditFull: true,
	}
	nc1, err := db.IssueNC(context.Background(), sig, p)
	if err != nil {
		t.Fatal(err)
	}
	nc2, err := db.IssueNC(context.Background(), sig, p)
	if err != nil {
		t.Fatal(err)
	}
	if !nc2.IdempotentHit || nc2.DocumentID != nc1.DocumentID {
		t.Fatal("same request_id idempotency failed")
	}

	p.RequestID = "nc-idem-2"
	nc3, err := db.IssueNC(context.Background(), sig, p)
	if err != nil {
		t.Fatal(err)
	}
	if !nc3.IdempotentHit || nc3.DocumentID != nc1.DocumentID {
		t.Fatal("same business_key idempotency failed")
	}
}

func TestIssueNCCreditedFullRejected(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	ftID := seedDemoWithNC(t, db, sig)
	_, err = db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-full-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Full", CreditFull: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-full-2", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Again partial", CreditFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "1.00"}},
	})
	if err != store.ErrCreditNotAllowed {
		t.Fatalf("want credit_not_allowed, got %v", err)
	}
}

func TestIssueNCPartialCredit(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	ftID := seedDemoWithNC(t, db, sig)

	nc, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-partial-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Partial", CreditFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "5.00"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if nc.InvoiceNo != "NC NC2026DEMO01/1" {
		t.Fatalf("invoice_no %s", nc.InvoiceNo)
	}
	var status, credited string
	_ = db.SQL.QueryRow(`SELECT document_status, credited_gross_total FROM invoices WHERE id=?`, ftID).
		Scan(&status, &credited)
	if status != string(domain.DocumentCreditedPartial) || credited != "5.00" {
		t.Fatalf("status=%s credited=%s", status, credited)
	}
}

func TestIssueNCAmountExceeded(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	ftID := seedDemoWithNC(t, db, sig)
	_, err = db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-over-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Too much", CreditFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "20.00"}},
	})
	if err != store.ErrCreditAmountExceeded {
		t.Fatalf("want exceeded, got %v", err)
	}
}

func TestIssueNCAllowedDocTypes(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = seedDemoWithNC(t, db, sig)

	for _, docType := range []string{"FS", "FR"} {
		docType := docType
		t.Run(docType, func(t *testing.T) {
			id := "inv-" + docType
			_, err := db.SQL.Exec(`INSERT INTO invoices (
				id, store_id, document_type, series_id, series_code, sequence_number, invoice_no,
				atcud, hash, hash_control, signing_key_version, previous_hash, qr_content,
				invoice_date, system_entry_date, document_status, print_status,
				gross_total, net_total, tax_payable, source_id, software_certificate_number,
				credited_gross_total, created_at
			) SELECT ?, store_id, ?, series_id, series_code, 99, ?, 'X-99', 'hash', 1, 1, '', '',
				'2026-08-20', '2026-08-20T12:00:00', 'SIGNED', 'PRINTED',
				'10.00', '8.13', '1.87', 'op-demo-cashier', '0', '0.00', datetime('now')
				FROM invoices WHERE document_type='FT' LIMIT 1`, id, docType, docType+" TEST/99")
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.SQL.Exec(`INSERT INTO invoice_lines (
				id, invoice_id, line_number, product_code, product_description, quantity, unit_of_measure,
				unit_price_gross, unit_price_net, line_gross, line_net, line_tax, vat_rate, tax_type, tax_country_region, tax_code, product_type
			) VALUES (?, ?, 1, 'X', 'Item', '1', 'UN', '10.00', '8.13', '10.00', '8.13', '1.87', '0.23', 'IVA', 'PT', 'NOR', 'P')`,
				"line-"+docType, id)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.SQL.Exec(`INSERT INTO invoice_customer_snapshots (
				invoice_id, customer_tax_id, company_name, address_detail, city, postal_code, country, account_id, self_billing_indicator
			) VALUES (?, '999999990', 'CF', 'x', 'x', 'x', 'PT', 'Desconhecido', 0)`, id)
			if err != nil {
				t.Fatal(err)
			}

			nc, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
				StoreID: "store-demo-001", RequestID: "nc-" + docType, OriginalInvoiceID: id,
				OperatorID: "op-demo-cashier", Reason: "Type test", CreditFull: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if nc.DocumentType != domain.DocumentNC {
				t.Fatalf("type %s", nc.DocumentType)
			}
		})
	}
}

func TestIssueNCPrintPayload(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	ftID := seedDemoWithNC(t, db, sig)
	nc, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-print-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Print test", CreditFull: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payloadJSON string
	err = db.SQL.QueryRow(`SELECT payload_json FROM local_print_jobs WHERE invoice_id=?`, nc.DocumentID).Scan(&payloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payloadJSON, `"original_invoice_no":"FT FT2026DEMO01/1"`) ||
		!strings.Contains(payloadJSON, `"credit_reason":"Print test"`) ||
		!strings.Contains(payloadJSON, `"document_type":"NC"`) {
		t.Fatalf("payload missing NC fields: %s", payloadJSON)
	}
}
