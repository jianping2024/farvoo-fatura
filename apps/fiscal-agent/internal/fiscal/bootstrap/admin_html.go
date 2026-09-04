package bootstrap

import _ "embed"

// Admin HTML for Local Fiscal (embedded; shared by fiscal-local and Agent).
// Feedback: ONLY FiscalUI.showToast for presentation; thrown errors → FiscalUI.reportError
// (withBusy / .catch(reportError)). Business helpers throw only — see .cursor/rules/fiscal-admin-error-toast.mdc.
//
//go:embed admin/index.html
var adminHTML string
