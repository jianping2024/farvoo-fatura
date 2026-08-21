package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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

// UpsertFiscalProductByCode is the ONLY product write used by bill sync ingest.
// Creates when missing; updates name/price/vat when changed; skips when identical.
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
	if in.TaxCode == "" {
		in.TaxCode = TaxCodeFromVATPercent(in.VATRate)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	var id, display, saft, price, vat, tax string
	err = d.SQL.QueryRow(`
		SELECT id, IFNULL(display_name,''), saft_name, unit_price_gross, vat_rate, tax_code
		FROM fiscal_products WHERE product_code = ?`, in.ProductCode).
		Scan(&id, &display, &saft, &price, &vat, &tax)
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
