package main

import (
	"fmt"
	"strings"
	"time"
)

const credentialReminderBeforeDays = 30

func parseValidUntil(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (c *config) validUntilTime() (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	return parseValidUntil(c.ValidUntil)
}

func (c *config) credentialDaysRemaining(now time.Time) (days int, ok bool) {
	until, valid := c.validUntilTime()
	if !valid {
		return 0, false
	}
	if !until.After(now) {
		return 0, true
	}
	d := int(until.Sub(now).Hours() / 24)
	if until.Sub(now) > time.Duration(d)*24*time.Hour {
		d++
	}
	return d, true
}

func (c *config) credentialInReminderWindow(now time.Time) bool {
	days, ok := c.credentialDaysRemaining(now)
	if !ok {
		return false
	}
	return days > 0 && days <= credentialReminderBeforeDays
}

func (c *config) credentialAboutLine(locale string, now time.Time) string {
	until, valid := c.validUntilTime()
	if !valid {
		return ""
	}
	loc := normalizeUILocale(locale)
	dateStr := until.Local().Format("2006-01-02")
	if !until.After(now) {
		return uiT(loc, "credential_expired_line")
	}
	if days, ok := c.credentialDaysRemaining(now); ok && days <= credentialReminderBeforeDays {
		return fmt.Sprintf(uiT(loc, "credential_days_left_line"), dateStr, days)
	}
	return fmt.Sprintf(uiT(loc, "credential_valid_until_line"), dateStr)
}

func (c *config) credentialStatusSuffix(locale string, now time.Time) string {
	if c == nil || !c.credentialInReminderWindow(now) {
		return ""
	}
	days, _ := c.credentialDaysRemaining(now)
	if days <= 0 {
		return ""
	}
	return fmt.Sprintf(uiT(normalizeUILocale(locale), "credential_status_suffix"), days)
}
