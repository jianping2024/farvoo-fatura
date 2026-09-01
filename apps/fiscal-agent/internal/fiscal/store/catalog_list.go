package store

import (
	"math"
	"strings"
)

// CatalogListQuery filters paginated product/customer catalog lists (Admin).
type CatalogListQuery struct {
	Page     int
	PageSize int
	Q        string
}

// ProductListResult is the paginated product list payload.
type ProductListResult struct {
	Items    []FiscalProductRow
	Total    int
	Page     int
	PageSize int
}

// CustomerListResult is the paginated customer list payload.
type CustomerListResult struct {
	Items    []CustomerRow
	Total    int
	Page     int
	PageSize int
}

var allowedCatalogPageSizes = map[int]bool{10: true, 20: true, 200: true, 500: true}

func normalizeCatalogListQuery(q CatalogListQuery) (CatalogListQuery, int, int) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if !allowedCatalogPageSizes[pageSize] {
		pageSize = 10
	}
	return CatalogListQuery{
		Page:     page,
		PageSize: pageSize,
		Q:        strings.TrimSpace(q.Q),
	}, page, pageSize
}

func catalogSearchLike(q string) string {
	return "%" + escapeLike(q) + "%"
}

// ListFiscalProductsPaged lists active products with optional search and pagination.
func (d *DB) ListFiscalProductsPaged(q CatalogListQuery) (*ProductListResult, error) {
	q, page, pageSize := normalizeCatalogListQuery(q)
	where := `FROM fiscal_products WHERE active = 1`
	args := []any{}
	if q.Q != "" {
		like := catalogSearchLike(q.Q)
		where += ` AND (product_code LIKE ? ESCAPE '\'
			OR IFNULL(display_name,'') LIKE ? ESCAPE '\'
			OR saft_name LIKE ? ESCAPE '\')`
		args = append(args, like, like, like)
	}

	var total int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	totalPages := int(math.Max(1, math.Ceil(float64(total)/float64(pageSize))))
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	selectQuery := `
		SELECT id, product_code, IFNULL(display_name,''), saft_name, unit_price_gross, vat_rate, tax_code, source, active ` +
		where + ` ORDER BY product_code LIMIT ? OFFSET ?`
	selectArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := d.SQL.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []FiscalProductRow{}
	for rows.Next() {
		var r FiscalProductRow
		if err := rows.Scan(&r.ID, &r.ProductCode, &r.DisplayName, &r.SaftName, &r.UnitPriceGross, &r.VATRate, &r.TaxCode, &r.Source, &r.Active); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ProductListResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListCustomersPaged lists customers with optional search and pagination.
func (d *DB) ListCustomersPaged(q CatalogListQuery) (*CustomerListResult, error) {
	q, page, pageSize := normalizeCatalogListQuery(q)
	where := `FROM customers WHERE 1=1`
	args := []any{}
	if q.Q != "" {
		like := catalogSearchLike(q.Q)
		where += ` AND (customer_tax_id LIKE ? ESCAPE '\'
			OR company_name LIKE ? ESCAPE '\')`
		args = append(args, like, like)
	}

	var total int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	totalPages := int(math.Max(1, math.Ceil(float64(total)/float64(pageSize))))
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	selectQuery := `
		SELECT id, customer_tax_id, company_name, address_detail, city, postal_code, country, completeness_status ` +
		where + ` ORDER BY company_name LIMIT ? OFFSET ?`
	selectArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := d.SQL.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []CustomerRow{}
	for rows.Next() {
		var r CustomerRow
		if err := rows.Scan(&r.ID, &r.CustomerTaxID, &r.CompanyName, &r.AddressDetail, &r.City, &r.PostalCode, &r.Country, &r.Completeness); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &CustomerListResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
