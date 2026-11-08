package bootstrap

import (
	"strings"
	"testing"
)

// SAF-T text errors: the Agent is the only validator; Admin maps saft_text_invalid to one
// mark/focus helper and one toast formatter, and no longer copies display_name into saft_name.
func TestAdminSAFTTextErrorSinglePath(t *testing.T) {
	html := string(adminHTML)
	if strings.Contains(html, "body.saft_name = body.display_name") {
		t.Fatal("saft_name fallback belongs to the Agent (store.CleanProductText), not Admin")
	}
	if n := strings.Count(html, "function focusSAFTTextField("); n != 1 {
		t.Fatalf("focusSAFTTextField must be defined once, got %d", n)
	}
	for _, ids := range []string{"PRODUCT_SAFT_FIELDS", "CUSTOMER_SAFT_FIELDS", "TAXPAYER_SAFT_FIELDS", "ORDER_SAFT_FIELDS"} {
		if !strings.Contains(html, "focusSAFTTextField(er, "+ids+")") {
			t.Fatalf("%s form must focus its saft_text_invalid field", ids)
		}
	}
	// Issue step shows #invSummaryLines (edit table hidden): a lines.<n> error marks both.
	if !strings.Contains(html, `'<tr data-line="' + idx + '"><td>' + (ln.name || ln.product_code)`) ||
		!strings.Contains(html, "#invSummaryLines ' + sel") {
		t.Fatal("lines.<n> errors must mark the issue-step summary row too")
	}
	toastJS := string(fiscalUIToastJS)
	if !strings.Contains(toastJS, "code === 'saft_text_invalid'") || strings.Count(toastJS, "function saftTextMessage(") != 1 {
		t.Fatal("toast.js formatError must own the saft_text_invalid message")
	}
	i18n := string(fiscalUIAdminI18nJS)
	for _, key := range []string{
		"errors.saft_text_invalid.encoding", "errors.saft_text_invalid.too_short", "errors.saft_text_invalid.too_long",
		"saft.line_prefix", "saft.saft_name_hint", "saft.field.saft_name", "saft.field.product_code",
		"saft.field.company_name", "saft.field.legal_name", "saft.field.address_detail", "saft.field.city",
		"saft.field.postal_code", "saft.field.reason", "product.fiscal_name_hint", "product.field.fiscal_name",
	} {
		if n := strings.Count(i18n, "'"+key+"':"); n != 3 {
			t.Fatalf("%s must exist in zh/en/pt, got %d", key, n)
		}
	}
	// Storefront copy: Chinese operators see「葡语名称」, not the opaque「税票名称」.
	if strings.Contains(i18n, "税票名称") || strings.Contains(html, "税票名称") {
		t.Fatal("user-facing Admin copy must say 葡语名称, not 税票名称")
	}
	if !strings.Contains(i18n, "'saft.field.saft_name': '葡语名称'") ||
		!strings.Contains(i18n, "'product.field.fiscal_name': '葡语名称'") ||
		!strings.Contains(i18n, "'saft.field.saft_name': 'Portuguese name'") ||
		!strings.Contains(i18n, "'saft.field.saft_name': 'Nome em português'") {
		t.Fatal("saft_name field labels must be 葡语名称 / Portuguese name / Nome em português")
	}
}
