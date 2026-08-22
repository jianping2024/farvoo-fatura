package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/google/uuid"
)

const consumidorFinalID = "cust-final"

// LocalCustomerInput is input for UpsertLocalCustomer (ONLY LOCAL customer write).
type LocalCustomerInput struct {
	CustomerTaxID string
	CompanyName   string
	AddressDetail string
	City          string
	PostalCode    string
	Country       string
}

// CustomerRow is a thin customer master row.
type CustomerRow struct {
	ID            string `json:"id"`
	CustomerTaxID string `json:"customer_tax_id"`
	CompanyName   string `json:"company_name"`
	AddressDetail string `json:"address_detail"`
	City          string `json:"city"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
	Completeness  string `json:"completeness_status"`
}

// UpsertLocalCustomer is the ONLY write path for operator-maintained customers.
// Refuses to mutate consumidor final (999999990).
func (d *DB) UpsertLocalCustomer(in LocalCustomerInput) (*CustomerRow, error) {
	tax := strings.TrimSpace(in.CustomerTaxID)
	if tax == "" {
		return nil, fmt.Errorf("store: customer_tax_id required")
	}
	if tax == "999999990" {
		return nil, fmt.Errorf("store: cannot upsert consumidor final via LOCAL API")
	}
	if len(tax) != 9 {
		return nil, fmt.Errorf("store: customer_tax_id must be 9 digits")
	}
	name := strings.TrimSpace(in.CompanyName)
	if name == "" {
		name = tax
	}
	addr := strings.TrimSpace(in.AddressDetail)
	if addr == "" {
		addr = "Desconhecido"
	}
	city := strings.TrimSpace(in.City)
	if city == "" {
		city = "Desconhecido"
	}
	postal := strings.TrimSpace(in.PostalCode)
	if postal == "" {
		postal = "Desconhecido"
	}
	country := strings.TrimSpace(in.Country)
	if country == "" {
		country = "PT"
	}
	now := time.Now().UTC().Format(time.RFC3339)

	var id string
	err := d.SQL.QueryRow(`SELECT id FROM customers WHERE customer_tax_id = ?`, tax).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		id = uuid.NewString()
		_, err = d.SQL.Exec(`INSERT INTO customers (
			id, customer_tax_id, company_name, address_detail, city, postal_code, country,
			account_id, self_billing_indicator, completeness_status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'Desconhecido', 0, 'LOCAL', ?, ?)`,
			id, tax, name, addr, city, postal, country, now, now)
		if err != nil {
			return nil, err
		}
		return &CustomerRow{
			ID: id, CustomerTaxID: tax, CompanyName: name,
			AddressDetail: addr, City: city, PostalCode: postal, Country: country,
			Completeness: "LOCAL",
		}, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = d.SQL.Exec(`UPDATE customers SET company_name = ?, address_detail = ?, city = ?,
		postal_code = ?, country = ?, completeness_status = 'LOCAL', updated_at = ? WHERE id = ?`,
		name, addr, city, postal, country, now, id)
	if err != nil {
		return nil, err
	}
	return &CustomerRow{
		ID: id, CustomerTaxID: tax, CompanyName: name,
		AddressDetail: addr, City: city, PostalCode: postal, Country: country,
		Completeness: "LOCAL",
	}, nil
}

// ListCustomers returns customers ordered by company name (excludes none).
func (d *DB) ListCustomers(limit int) ([]CustomerRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := d.SQL.Query(`
		SELECT id, customer_tax_id, company_name, address_detail, city, postal_code, country, completeness_status
		FROM customers ORDER BY company_name LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomerRow
	for rows.Next() {
		var r CustomerRow
		if err := rows.Scan(&r.ID, &r.CustomerTaxID, &r.CompanyName, &r.AddressDetail, &r.City, &r.PostalCode, &r.Country, &r.Completeness); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetCustomerByTaxID loads one customer by NIF.
func (d *DB) GetCustomerByTaxID(taxID string) (*CustomerRow, error) {
	taxID = strings.TrimSpace(taxID)
	var r CustomerRow
	err := d.SQL.QueryRow(`
		SELECT id, customer_tax_id, company_name, address_detail, city, postal_code, country, completeness_status
		FROM customers WHERE customer_tax_id = ?`, taxID).
		Scan(&r.ID, &r.CustomerTaxID, &r.CompanyName, &r.AddressDetail, &r.City, &r.PostalCode, &r.Country, &r.Completeness)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ensureCustomerIDTx resolves invoices.customer_id for a normalized buyer snapshot.
func (d *DB) ensureCustomerIDTx(tx *sql.Tx, c domain.CustomerInput) (string, error) {
	tax := strings.TrimSpace(c.TaxID)
	if tax == "" || tax == "999999990" {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO customers (
			id, customer_tax_id, company_name, address_detail, city, postal_code, country,
			account_id, self_billing_indicator, completeness_status, created_at, updated_at
		) VALUES ('cust-final', '999999990', 'Consumidor Final', 'Desconhecido', 'Desconhecido', 'Desconhecido', 'PT',
			'Desconhecido', 0, 'SYSTEM_DEFAULT', datetime('now'), datetime('now'))`); err != nil {
			return "", err
		}
		return consumidorFinalID, nil
	}
	var id string
	err := tx.QueryRow(`SELECT id FROM customers WHERE customer_tax_id = ?`, tax).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	name := strings.TrimSpace(c.CompanyName)
	if name == "" {
		name = tax
	}
	_, err = tx.Exec(`INSERT INTO customers (
		id, customer_tax_id, company_name, address_detail, city, postal_code, country,
		account_id, self_billing_indicator, completeness_status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'Desconhecido', 0, 'LOCAL', ?, ?)`,
		id, tax, name, c.AddressDetail, c.City, c.PostalCode, c.Country, now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}
