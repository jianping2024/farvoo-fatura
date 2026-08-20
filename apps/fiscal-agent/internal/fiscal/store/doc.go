// Package store is the SQLite persistence layer for fiscal authority data.
//
// Design: docs/fiscal-sqlite-schema.zh.md
// DDL:    store/migrations/001_init.sql
//
// Concurrency: BEGIN IMMEDIATE per series for issue transactions.
// Amounts: TEXT decimal strings only — never REAL/float.
package store
