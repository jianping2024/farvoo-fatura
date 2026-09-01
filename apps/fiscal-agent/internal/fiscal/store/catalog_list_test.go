package store

import (
	"fmt"
	"testing"
)

func TestListFiscalProductsPagedSearchAndPagination(t *testing.T) {
	db := openTestDB(t)
	for i := 1; i <= 12; i++ {
		code := fmt.Sprintf("P%02d", i)
		if _, err := db.UpsertLocalFiscalProduct(LocalProductInput{
			ProductCode:    code,
			DisplayName:    "Item " + code,
			SaftName:       "Item " + code,
			UnitPriceGross: "1.00",
			VATRate:        "13.00",
		}); err != nil {
			t.Fatal(err)
		}
	}

	page1, err := db.ListFiscalProductsPaged(CatalogListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 12 || len(page1.Items) != 10 || page1.Items[0].ProductCode != "P01" {
		t.Fatalf("page1: total=%d items=%d first=%s", page1.Total, len(page1.Items), page1.Items[0].ProductCode)
	}

	page2, err := db.ListFiscalProductsPaged(CatalogListQuery{Page: 2, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 2 || page2.Items[0].ProductCode != "P11" {
		t.Fatalf("page2: %+v", page2.Items)
	}

	byP12, err := db.ListFiscalProductsPaged(CatalogListQuery{Page: 1, PageSize: 10, Q: "P12"})
	if err != nil {
		t.Fatal(err)
	}
	if byP12.Total != 1 || len(byP12.Items) != 1 || byP12.Items[0].ProductCode != "P12" {
		t.Fatalf("search: %+v", byP12)
	}
}

func TestListCustomersPagedSearch(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.UpsertLocalCustomer(LocalCustomerInput{
		CustomerTaxID: "123456789",
		CompanyName:   "Alpha Lda",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertLocalCustomer(LocalCustomerInput{
		CustomerTaxID: "517535009",
		CompanyName:   "Beta SA",
	}); err != nil {
		t.Fatal(err)
	}

	all, err := db.ListCustomersPaged(CatalogListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total < 2 {
		t.Fatalf("expected at least 2 customers, got %d", all.Total)
	}

	alpha, err := db.ListCustomersPaged(CatalogListQuery{Page: 1, PageSize: 10, Q: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if alpha.Total != 1 || alpha.Items[0].CompanyName != "Alpha Lda" {
		t.Fatalf("search alpha: %+v", alpha)
	}
}
