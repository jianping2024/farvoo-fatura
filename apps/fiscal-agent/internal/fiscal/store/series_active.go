package store

// HasActiveSeries reports whether store has an ACTIVE series with validation_code for docType.
func (d *DB) HasActiveSeries(storeID, docType string) (bool, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(1) FROM series
		WHERE store_id = ? AND document_type = ? AND status = 'ACTIVE'
		AND validation_code IS NOT NULL AND validation_code != ''`,
		storeID, docType).Scan(&n)
	return n > 0, err
}
