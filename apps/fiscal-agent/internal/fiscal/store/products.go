package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/vatpercent"

	"github.com/google/uuid"
)

// ProductUpsertInput is a thin catalog row from bill sync lines (ONLY upsert path for REMOTE_SYNC from sync).
type ProductUpsertInput struct {
	ProductCode    string
	DisplayName    string
	SaftName       string
	UnitPriceGross string
	VATRate        string // percent "13.00"
	TaxCode        string
	RemoteItemID   string
}

// FiscalProductRow is a catalog row for API/list.
type FiscalProductRow struct {
	ID             string `json:"id"`
	ProductCode    string `json:"product_code"`
	DisplayName    string `json:"display_name"`
	SaftName       string `json:"saft_name"`
	UnitPriceGross string `json:"unit_price_gross"`
	VATRate        string `json:"vat_rate"`
	TaxCode        string `json:"tax_code"`
	Source         string `json:"source"`
	Active         int    `json:"active"`
}

// LocalProductInput is input for UpsertLocalFiscalProduct (ONLY LOCAL product write).
type LocalProductInput struct {
	ProductCode    string
	DisplayName    string
	SaftName       string
	UnitPriceGross string
	VATRate        string // percent "23.00"
	TaxCode        string
}

// UpsertFiscalProductByCode is the ONLY product write used by bill sync ingest (REMOTE_SYNC).
// Never overwrites LOCAL rows; refuses to create when code owned by LOCAL.
func (d *DB) UpsertFiscalProductByCode(in ProductUpsertInput) (created, updated bool, err error) {
	in.ProductCode = strings.TrimSpace(in.ProductCode)
	if in.ProductCode == "" {
		return false, false, fmt.Errorf("store: product_code required")
	}
	if strings.TrimSpace(in.SaftName) == "" {
		in.SaftName = in.DisplayName
	}
	if strings.TrimSpace(in.SaftName) == "" {
		return false, false, fmt.Errorf("store: saft_name required")
	}
	if strings.TrimSpace(in.UnitPriceGross) == "" || strings.TrimSpace(in.VATRate) == "" {
		return false, false, fmt.Errorf("store: unit_price_gross and vat_rate required")
	}
	normVAT, err := vatpercent.Normalize(in.VATRate)
	if err != nil {
		return false, false, err
	}
	in.VATRate = normVAT
	if in.TaxCode == "" {
		in.TaxCode = TaxCodeFromVATPercent(in.VATRate)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	var id, display, saft, price, vat, tax, source string
	err = d.SQL.QueryRow(`
		SELECT id, IFNULL(display_name,''), saft_name, unit_price_gross, vat_rate, tax_code, source
		FROM fiscal_products WHERE product_code = ?`, in.ProductCode).
		Scan(&id, &display, &saft, &price, &vat, &tax, &source)
	if errors.Is(err, sql.ErrNoRows) {
		id = uuid.NewString()
		_, err = d.SQL.Exec(`
			INSERT INTO fiscal_products(
				id, product_code, category_id, display_name, name_pt, name_en, saft_name,
				product_type, unit_of_measure, unit_price_gross, vat_rate, tax_code,
				source, remote_item_id, active, created_at, updated_at
			) VALUES (?, ?, NULL, ?, ?, NULL, ?, 'P', 'UN', ?, ?, ?, 'REMOTE_SYNC', ?, 1, ?, ?)`,
			id, in.ProductCode, nullStr(in.DisplayName), nullStr(in.SaftName), in.SaftName,
			in.UnitPriceGross, in.VATRate, in.TaxCode, nullStr(in.RemoteItemID), now, now)
		if err != nil {
			return false, false, err
		}
		return true, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if source == "LOCAL" {
		return false, false, nil
	}

	same := display == strings.TrimSpace(in.DisplayName) &&
		saft == strings.TrimSpace(in.SaftName) &&
		price == strings.TrimSpace(in.UnitPriceGross) &&
		vat == strings.TrimSpace(in.VATRate)
	if same {
		return false, false, nil
	}
	_, err = d.SQL.Exec(`
		UPDATE fiscal_products SET display_name = ?, name_pt = ?, saft_name = ?,
			unit_price_gross = ?, vat_rate = ?, tax_code = ?, updated_at = ?
		WHERE product_code = ?`,
		nullStr(in.DisplayName), nullStr(in.SaftName), in.SaftName,
		in.UnitPriceGross, in.VATRate, in.TaxCode, now, in.ProductCode)
	if err != nil {
		return false, false, err
	}
	return false, true, nil
}

// UpsertLocalFiscalProduct is the ONLY write path for operator-maintained products.
func (d *DB) UpsertLocalFiscalProduct(in LocalProductInput) (*FiscalProductRow, error) {
	in.ProductCode = strings.TrimSpace(in.ProductCode)
	if in.ProductCode == "" {
		return nil, fmt.Errorf("store: product_code required")
	}
	if strings.TrimSpace(in.SaftName) == "" {
		in.SaftName = in.DisplayName
	}
	if strings.TrimSpace(in.SaftName) == "" {
		return nil, fmt.Errorf("store: saft_name required")
	}
	if strings.TrimSpace(in.UnitPriceGross) == "" || strings.TrimSpace(in.VATRate) == "" {
		return nil, fmt.Errorf("store: unit_price_gross and vat_rate required")
	}
	normVAT, err := vatpercent.Normalize(in.VATRate)
	if err != nil {
		return nil, err
	}
	in.VATRate = normVAT
	if in.TaxCode == "" {
		in.TaxCode = TaxCodeFromVATPercent(in.VATRate)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	var id, source string
	err = d.SQL.QueryRow(`SELECT id, source FROM fiscal_products WHERE product_code = ?`, in.ProductCode).Scan(&id, &source)
	if err == nil && source == "REMOTE_SYNC" {
		return nil, fmt.Errorf("store: product_code %q owned by REMOTE_SYNC", in.ProductCode)
	}
	if errors.Is(err, sql.ErrNoRows) {
		id = uuid.NewString()
		_, err = d.SQL.Exec(`
			INSERT INTO fiscal_products(
				id, product_code, category_id, display_name, name_pt, name_en, saft_name,
				product_type, unit_of_measure, unit_price_gross, vat_rate, tax_code,
				source, remote_item_id, active, created_at, updated_at
			) VALUES (?, ?, NULL, ?, ?, NULL, ?, 'P', 'UN', ?, ?, ?, 'LOCAL', NULL, 1, ?, ?)`,
			id, in.ProductCode, nullStr(in.DisplayName), nullStr(in.SaftName), in.SaftName,
			in.UnitPriceGross, in.VATRate, in.TaxCode, now, now)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		_, err = d.SQL.Exec(`
			UPDATE fiscal_products SET display_name = ?, name_pt = ?, saft_name = ?,
				unit_price_gross = ?, vat_rate = ?, tax_code = ?, source = 'LOCAL', updated_at = ?
			WHERE product_code = ? AND source = 'LOCAL'`,
			nullStr(in.DisplayName), nullStr(in.SaftName), in.SaftName,
			in.UnitPriceGross, in.VATRate, in.TaxCode, now, in.ProductCode)
		if err != nil {
			return nil, err
		}
	}
	return d.GetFiscalProductByCode(in.ProductCode)
}

// ListFiscalProducts lists active catalog rows.
func (d *DB) ListFiscalProducts(limit int) ([]FiscalProductRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := d.SQL.Query(`
		SELECT id, product_code, IFNULL(display_name,''), saft_name, unit_price_gross, vat_rate, tax_code, source, active
		FROM fiscal_products WHERE active = 1 ORDER BY product_code LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FiscalProductRow
	for rows.Next() {
		var r FiscalProductRow
		if err := rows.Scan(&r.ID, &r.ProductCode, &r.DisplayName, &r.SaftName, &r.UnitPriceGross, &r.VATRate, &r.TaxCode, &r.Source, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetFiscalProductByCode loads one product by code.
func (d *DB) GetFiscalProductByCode(code string) (*FiscalProductRow, error) {
	code = strings.TrimSpace(code)
	var r FiscalProductRow
	err := d.SQL.QueryRow(`
		SELECT id, product_code, IFNULL(display_name,''), saft_name, unit_price_gross, vat_rate, tax_code, source, active
		FROM fiscal_products WHERE product_code = ?`, code).
		Scan(&r.ID, &r.ProductCode, &r.DisplayName, &r.SaftName, &r.UnitPriceGross, &r.VATRate, &r.TaxCode, &r.Source, &r.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// TaxCodeFromVATPercent maps percent strings to SAF-T tax codes.
func TaxCodeFromVATPercent(vatPercent string) string {
	v := strings.TrimSpace(vatPercent)
	switch {
	case v == "0.00" || v == "0":
		return "ISE"
	case strings.HasPrefix(v, "6"):
		return "RED"
	case strings.HasPrefix(v, "13"):
		return "INT"
	default:
		return "NOR"
	}
}
