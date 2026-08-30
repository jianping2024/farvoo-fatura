package store

import (
	"fmt"
	"time"
)

// SAFTPeriodBounds returns inclusive YYYY-MM-DD start/end for a calendar month in store timezone.
func SAFTPeriodBounds(year, month int, timezone string) (startDate, endDate string, err error) {
	if year < 2000 || month < 1 || month > 12 {
		return "", "", fmt.Errorf("store: invalid saft period %d-%02d", year, month)
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, -1)
	return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}
