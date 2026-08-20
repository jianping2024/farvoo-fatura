// Package compliance implements Portugal AT rules without business orchestration.
//
// Subpackages / files (to be added):
//   - hash.go      RSA-SHA1 sign/verify per Despacho 8632/2014
//   - atcud.go     SeriesValidationCode + sequence formatting
//   - qrcode.go    QR field A–Q assembly
//   - nif.go       Customer NIF checksum
//   - encoding.go  Windows-1252 validation for SAF-T text fields
//
// Must not import store/, service/, or api/.
package compliance
