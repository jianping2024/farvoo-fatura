package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// saft_text_invalid carries field / part / limits so Admin can name and focus the input.
func TestUpsertProduct_SAFTTextErrorShape(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	body := `{"product_code":"jincha","display_name":"金叉套餐","unit_price_gross":"10.00","vat_rate":"23.00"}`
	req := httptest.NewRequest(http.MethodPost, "/local/v1/products", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handleUpsertProduct(rr, req, HandlerDeps{Fiscal: service.New(db, nil, nil, dir, "store-demo-001")})
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if rr.Code != http.StatusBadRequest || got["error"] != "saft_text_invalid" || got["field"] != "product.saft_name" ||
		got["part"] != "encoding" || got["min"] != float64(2) || got["max"] != float64(200) {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}
