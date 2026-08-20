package main

// onConfigureWizardReady is set by the Windows tray while settings run in-process.
var onConfigureWizardReady func(url string)
