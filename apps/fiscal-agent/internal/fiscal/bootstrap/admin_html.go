package bootstrap

import _ "embed"

// Admin HTML for Local Fiscal (embedded; shared by fiscal-local and Agent).
// Feedback: ONLY FiscalUI.showToast (ui/toast.js) — restaurant-ordering Toast contract.
//
//go:embed admin/index.html
var adminHTML string
