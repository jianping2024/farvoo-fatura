-- Fiscal Agent SQLite schema P0
-- Source of truth (design): docs/fiscal-sqlite-schema.zh.md
-- Amounts/quantities/rates: TEXT decimal strings. Never REAL.

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS taxpayer_settings (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL UNIQUE,
  tax_registration_number TEXT NOT NULL,
  legal_name TEXT NOT NULL,
  business_name TEXT,
  address_detail TEXT NOT NULL,
  city TEXT NOT NULL,
  postal_code TEXT NOT NULL,
  country TEXT NOT NULL DEFAULT 'PT',
  timezone TEXT NOT NULL DEFAULT 'Europe/Lisbon',
  phone TEXT,
  software_certificate_number TEXT NOT NULL DEFAULT '0',
  product_id TEXT NOT NULL DEFAULT 'Farvoo/InvoiceEngine',
  product_version TEXT NOT NULL DEFAULT '0.0.0',
  fs_amount_threshold TEXT NOT NULL DEFAULT '100.00',
  tax_country_region TEXT NOT NULL DEFAULT 'PT',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS at_credentials (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL UNIQUE,
  username TEXT NOT NULL,
  password_ciphertext TEXT NOT NULL,
  salt TEXT,
  wrap_meta TEXT,
  last_ok_at TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS signing_keys (
  id TEXT PRIMARY KEY NOT NULL,
  key_version INTEGER NOT NULL UNIQUE,
  public_key_pem TEXT NOT NULL,
  wrapped_private_key TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  retired_at TEXT,
  submitted_to_at_at TEXT
);

CREATE TABLE IF NOT EXISTS agent_installations (
  installation_id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  taxpayer_nif TEXT NOT NULL,
  device_id TEXT NOT NULL,
  device_public_key TEXT NOT NULL,
  hardware_fingerprint TEXT,
  key_protection_level TEXT NOT NULL,
  signing_key_version INTEGER NOT NULL,
  provisioned_at TEXT NOT NULL,
  revoked_at TEXT
);

CREATE TABLE IF NOT EXISTS operators (
  id TEXT PRIMARY KEY NOT NULL,
  mesa_user_id TEXT NOT NULL UNIQUE,
  store_id TEXT NOT NULL,
  role TEXT NOT NULL,
  display_name TEXT NOT NULL,
  active INTEGER NOT NULL DEFAULT 1,
  pin_hash TEXT,
  can_issue_nc INTEGER NOT NULL DEFAULT 0,
  synced_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS customers (
  id TEXT PRIMARY KEY NOT NULL,
  customer_tax_id TEXT NOT NULL,
  company_name TEXT NOT NULL,
  address_detail TEXT NOT NULL DEFAULT 'Desconhecido',
  city TEXT NOT NULL DEFAULT 'Desconhecido',
  postal_code TEXT NOT NULL DEFAULT 'Desconhecido',
  country TEXT NOT NULL DEFAULT 'PT',
  account_id TEXT NOT NULL DEFAULT 'Desconhecido',
  self_billing_indicator INTEGER NOT NULL DEFAULT 0,
  completeness_status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_tax_id_non_final
  ON customers(customer_tax_id)
  WHERE customer_tax_id != '999999990';

CREATE TABLE IF NOT EXISTS fiscal_product_categories (
  id TEXT PRIMARY KEY NOT NULL,
  parent_id TEXT,
  name_zh TEXT,
  name_pt TEXT,
  name_en TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  source TEXT NOT NULL,
  active INTEGER NOT NULL DEFAULT 1,
  FOREIGN KEY (parent_id) REFERENCES fiscal_product_categories(id)
);

CREATE TABLE IF NOT EXISTS fiscal_products (
  id TEXT PRIMARY KEY NOT NULL,
  product_code TEXT NOT NULL UNIQUE,
  category_id TEXT,
  display_name TEXT,
  name_pt TEXT,
  name_en TEXT,
  saft_name TEXT NOT NULL,
  product_type TEXT NOT NULL DEFAULT 'P',
  unit_of_measure TEXT NOT NULL DEFAULT 'UN',
  unit_price_gross TEXT NOT NULL,
  vat_rate TEXT NOT NULL,
  tax_code TEXT NOT NULL,
  source TEXT NOT NULL,
  remote_item_id TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (category_id) REFERENCES fiscal_product_categories(id)
);

CREATE TABLE IF NOT EXISTS series (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  document_type TEXT NOT NULL,
  series_code TEXT NOT NULL,
  validation_code TEXT,
  fiscal_year INTEGER NOT NULL,
  last_number INTEGER NOT NULL DEFAULT 0,
  last_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  registered_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (store_id, series_code)
);

CREATE INDEX IF NOT EXISTS idx_series_store_type_year
  ON series(store_id, document_type, fiscal_year);

CREATE TABLE IF NOT EXISTS invoices (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  document_type TEXT NOT NULL,
  series_id TEXT NOT NULL,
  series_code TEXT NOT NULL,
  sequence_number INTEGER NOT NULL,
  invoice_no TEXT NOT NULL UNIQUE,
  atcud TEXT NOT NULL,
  hash TEXT NOT NULL,
  hash_control INTEGER NOT NULL,
  signing_key_version INTEGER NOT NULL,
  previous_hash TEXT NOT NULL DEFAULT '',
  qr_content TEXT NOT NULL,
  invoice_date TEXT NOT NULL,
  system_entry_date TEXT NOT NULL,
  document_status TEXT NOT NULL,
  print_status TEXT NOT NULL,
  gross_total TEXT NOT NULL,
  net_total TEXT NOT NULL,
  tax_payable TEXT NOT NULL,
  customer_id TEXT,
  source_id TEXT NOT NULL,
  software_certificate_number TEXT NOT NULL,
  source_system TEXT,
  source_sale_id TEXT,
  scope_type TEXT,
  scope_id TEXT,
  fiscal_purpose TEXT,
  external_bill_id TEXT,
  display_meta_json TEXT,
  credited_gross_total TEXT NOT NULL DEFAULT '0.00',
  created_at TEXT NOT NULL,
  FOREIGN KEY (series_id) REFERENCES series(id),
  FOREIGN KEY (customer_id) REFERENCES customers(id),
  FOREIGN KEY (source_id) REFERENCES operators(id)
);

CREATE INDEX IF NOT EXISTS idx_invoices_store_date
  ON invoices(store_id, invoice_date);

CREATE INDEX IF NOT EXISTS idx_invoices_business_scope
  ON invoices(store_id, source_system, source_sale_id, scope_type, scope_id, fiscal_purpose);

CREATE TABLE IF NOT EXISTS invoice_lines (
  id TEXT PRIMARY KEY NOT NULL,
  invoice_id TEXT NOT NULL,
  line_number INTEGER NOT NULL,
  product_code TEXT NOT NULL,
  product_description TEXT NOT NULL,
  display_name TEXT,
  quantity TEXT NOT NULL,
  unit_of_measure TEXT NOT NULL DEFAULT 'UN',
  unit_price_gross TEXT NOT NULL,
  unit_price_net TEXT NOT NULL,
  line_gross TEXT NOT NULL,
  line_net TEXT NOT NULL,
  line_tax TEXT NOT NULL,
  vat_rate TEXT NOT NULL,
  tax_type TEXT NOT NULL DEFAULT 'IVA',
  tax_country_region TEXT NOT NULL DEFAULT 'PT',
  tax_code TEXT NOT NULL,
  tax_exemption_code TEXT,
  tax_exemption_reason TEXT,
  product_type TEXT NOT NULL DEFAULT 'P',
  UNIQUE (invoice_id, line_number),
  FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);

CREATE TABLE IF NOT EXISTS invoice_customer_snapshots (
  invoice_id TEXT PRIMARY KEY NOT NULL,
  customer_tax_id TEXT NOT NULL,
  company_name TEXT NOT NULL,
  address_detail TEXT NOT NULL,
  city TEXT NOT NULL,
  postal_code TEXT NOT NULL,
  country TEXT NOT NULL,
  account_id TEXT NOT NULL DEFAULT 'Desconhecido',
  self_billing_indicator INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);

CREATE TABLE IF NOT EXISTS invoice_payments (
  id TEXT PRIMARY KEY NOT NULL,
  invoice_id TEXT NOT NULL,
  method TEXT NOT NULL,
  amount TEXT NOT NULL,
  paid_at TEXT NOT NULL,
  operator_id TEXT,
  FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);

CREATE TABLE IF NOT EXISTS invoice_line_references (
  id TEXT PRIMARY KEY NOT NULL,
  credit_line_id TEXT NOT NULL UNIQUE,
  original_invoice_id TEXT NOT NULL,
  original_invoice_no TEXT NOT NULL,
  original_line_id TEXT NOT NULL,
  original_line_number INTEGER NOT NULL,
  reason TEXT NOT NULL,
  FOREIGN KEY (credit_line_id) REFERENCES invoice_lines(id),
  FOREIGN KEY (original_invoice_id) REFERENCES invoices(id)
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  request_payload_hash TEXT NOT NULL,
  business_key TEXT NOT NULL,
  invoice_id TEXT,
  created_at TEXT NOT NULL,
  UNIQUE (store_id, request_id),
  UNIQUE (store_id, business_key),
  FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);

CREATE TABLE IF NOT EXISTS local_print_jobs (
  id TEXT PRIMARY KEY NOT NULL,
  invoice_id TEXT NOT NULL,
  document_type TEXT NOT NULL,
  print_purpose TEXT NOT NULL,
  job_status TEXT NOT NULL,
  logical_role TEXT NOT NULL DEFAULT 'fiscal_receipt_printer',
  payload_json TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  printed_at TEXT,
  created_by TEXT,
  FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);

CREATE INDEX IF NOT EXISTS idx_local_print_jobs_status
  ON local_print_jobs(job_status, created_at);

CREATE TABLE IF NOT EXISTS print_attempts (
  id TEXT PRIMARY KEY NOT NULL,
  print_job_id TEXT NOT NULL,
  attempted_at TEXT NOT NULL,
  result TEXT NOT NULL,
  error_code TEXT,
  error_message TEXT,
  device_hint TEXT,
  FOREIGN KEY (print_job_id) REFERENCES local_print_jobs(id)
);

CREATE TABLE IF NOT EXISTS audit_log (
  id TEXT PRIMARY KEY NOT NULL,
  at TEXT NOT NULL,
  operator_id TEXT,
  action TEXT NOT NULL,
  entity_type TEXT,
  entity_id TEXT,
  detail_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_log_at ON audit_log(at);

CREATE TABLE IF NOT EXISTS saft_exports (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  taxpayer_nif TEXT NOT NULL,
  period_year INTEGER NOT NULL,
  period_month INTEGER NOT NULL,
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL,
  file_name TEXT NOT NULL,
  file_path TEXT,
  file_sha256 TEXT,
  invoice_count INTEGER NOT NULL DEFAULT 0,
  total_net TEXT,
  total_tax TEXT,
  total_gross TEXT,
  validation_status TEXT NOT NULL,
  validation_errors TEXT,
  created_by TEXT,
  created_at TEXT NOT NULL,
  submitted_at TEXT,
  at_receipt_number TEXT,
  at_receipt_file_path TEXT
);

CREATE TABLE IF NOT EXISTS sync_outbox (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  sent_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_sync_outbox_pending
  ON sync_outbox(status, next_attempt_at);
