// Package at wraps Autoridade Tributária SOAP for series registration and
// Documentos de Transporte (GR/GT).
//
// Environments: FISCAL_AT_ENV = mock | test | prod.
// Default/CI is mock (no network). test/prod share EncryptATAuth + LoadClientTLS
// (A0); live HTTP for series (A1) and transport (A2) builds on that base.
//
// Unique live helpers (A0):
//   - EncryptATAuth / LoadPasswordPublicKey
//   - LoadClientTLS
//   - BuildUsernameTokenXML / WrapSOAPEnvelope
//   - LiveMaterialsFromEnv
package at
