-- GR/GT load/unload points as structured addresses (AT AddressStructurePT, SAF-T ShipFrom/ShipTo).
-- Legacy ship_from_address / ship_to_address keep pre-017 free text; new rows leave them NULL.
ALTER TABLE invoices ADD COLUMN ship_from_detail TEXT;
ALTER TABLE invoices ADD COLUMN ship_from_city TEXT;
ALTER TABLE invoices ADD COLUMN ship_from_postal_code TEXT;
ALTER TABLE invoices ADD COLUMN ship_from_country TEXT;
ALTER TABLE invoices ADD COLUMN ship_to_detail TEXT;
ALTER TABLE invoices ADD COLUMN ship_to_city TEXT;
ALTER TABLE invoices ADD COLUMN ship_to_postal_code TEXT;
ALTER TABLE invoices ADD COLUMN ship_to_country TEXT;
