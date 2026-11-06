// Command saft-text-scan lists stored text that SAF-T cannot hold, before a release that turns on
// entry validation (domain.ValidateSAFTText — the same rule, not a copy). Read-only.
//
//	cd apps/fiscal-agent && go run ./cmd/saft-text-scan path/to/fiscal.db [more.db …]
//
// "fix" rows are master data the store must correct before issuing with them (product, customer,
// store settings); "signed" rows are frozen documents, listed only so their SAF-T months are known.
// Exit 1 when any "fix" row exists.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"

	_ "modernc.org/sqlite"
)

type column struct {
	name     string
	min, max int
}

type source struct {
	kind   string // fix | signed
	table  string
	key    string // SQL expression naming the row
	fields []column
}

var sources = []source{
	{"fix", "fiscal_products", "product_code", []column{
		{"product_code", 1, domain.SAFTMaxProductCode}, {"saft_name", domain.SAFTMinProductDesc, domain.SAFTMaxProductDesc}}},
	{"fix", "customers", "customer_tax_id", []column{
		{"company_name", 0, domain.SAFTMaxCompanyName}, {"address_detail", 0, domain.SAFTMaxAddressDetail},
		{"city", 0, domain.SAFTMaxCity}, {"postal_code", 0, domain.SAFTMaxPostalCode}}},
	{"fix", "taxpayer_settings", "tax_registration_number", []column{
		{"legal_name", 1, domain.SAFTMaxCompanyName}, {"business_name", 0, domain.SAFTMaxBusinessName},
		{"address_detail", 1, domain.SAFTMaxAddressDetail}, {"city", 1, domain.SAFTMaxCity}, {"postal_code", 1, domain.SAFTMaxPostalCode}}},
	{"signed", "invoice_lines", "(SELECT invoice_no FROM invoices WHERE id = invoice_id) || ' line ' || line_number", []column{
		{"product_code", 1, domain.SAFTMaxProductCode}, {"product_description", domain.SAFTMinProductDesc, domain.SAFTMaxProductDesc}}},
	{"signed", "invoice_customer_snapshots", "(SELECT invoice_no FROM invoices WHERE id = invoice_id)", []column{
		{"company_name", 0, domain.SAFTMaxCompanyName}, {"address_detail", 0, domain.SAFTMaxAddressDetail},
		{"city", 0, domain.SAFTMaxCity}, {"postal_code", 0, domain.SAFTMaxPostalCode}}},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: saft-text-scan fiscal.db [more.db …]")
		os.Exit(2)
	}
	fix := 0
	for _, path := range os.Args[1:] {
		n, err := scan(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(2)
		}
		fix += n
	}
	if fix > 0 {
		os.Exit(1)
	}
}

// scan prints every failing value of one database and returns the number of "fix" rows.
func scan(path string) (int, error) {
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	fix, signed := 0, 0
	for _, src := range sources {
		cols := make([]string, len(src.fields))
		for i, c := range src.fields {
			cols[i] = "COALESCE(" + c.name + ",'')"
		}
		rows, err := db.Query("SELECT " + src.key + ", " + strings.Join(cols, ", ") + " FROM " + src.table)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				continue
			}
			return 0, fmt.Errorf("%s: %w", src.table, err)
		}
		for rows.Next() {
			var key sql.NullString
			vals := make([]string, len(src.fields))
			dest := []any{&key}
			for i := range vals {
				dest = append(dest, &vals[i])
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return 0, err
			}
			for i, c := range src.fields {
				_, err := domain.ValidateSAFTText(c.name, vals[i], c.min, c.max)
				var te *domain.SAFTTextError
				if !errors.As(err, &te) {
					continue
				}
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%q\n", path, src.kind, src.table, key.String, c.name, te.Reason, vals[i])
				if src.kind == "fix" {
					fix++
				} else {
					signed++
				}
			}
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %d to fix, %d in signed documents\n", path, fix, signed)
	return fix, nil
}
