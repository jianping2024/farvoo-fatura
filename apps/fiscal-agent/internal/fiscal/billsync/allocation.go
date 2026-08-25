package billsync

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/google/uuid"
)

// Allocation is the ONLY mutable local by-item split plan (not cloud writeback).
// Shares reference SourceLines[].LineKey.
type Allocation struct {
	People []AllocPerson `json:"people"`
}

// AllocPerson is one diner share bucket.
type AllocPerson struct {
	ScopeID string       `json:"scope_id"`
	Name    string       `json:"name"`
	Shares  []AllocShare `json:"shares"`
}

// AllocShare is one line quantity for a person.
type AllocShare struct {
	LineKey string   `json:"line_key"`
	Qty     Rational `json:"qty"`
}

// LineKeyFor builds a stable key when cloud has no line id.
func LineKeyFor(ln Line, index int) string {
	if k := strings.TrimSpace(ln.LineKey); k != "" {
		return k
	}
	code := strings.TrimSpace(ln.ItemCode)
	vat := strings.TrimSpace(ln.VATRate)
	price := strings.TrimSpace(ln.UnitPriceGross)
	return fmt.Sprintf("%s|%s|%s|#%d", code, vat, price, index)
}

// PersonKey normalizes display name for duplicate checks (case-insensitive trim).
func PersonKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// FreezeSourceLines sets snap.SourceLines from commercial lines (ingest ONLY path helper).
// Mutates snap; does not write DB. No-op if already frozen.
// whole_table: one frozen row per payload line. split: aggregate split lines by identity + sum qty.
func FreezeSourceLines(snap *Snapshot) error {
	if snap == nil {
		return ingestErr(CodeValidationFailed, "snapshot nil")
	}
	if len(snap.SourceLines) > 0 {
		return nil
	}
	raw, err := CollectLines(*snap)
	if err != nil {
		return err
	}
	if strings.TrimSpace(snap.ScopeType) == "whole_table" {
		out := make([]Line, 0, len(raw))
		for i, ln := range raw {
			cp := ln
			cp.LineKey = LineKeyFor(ln, i)
			if strings.TrimSpace(cp.Qty) == "" {
				cp.Qty = "1"
			}
			out = append(out, cp)
		}
		snap.SourceLines = out
		return nil
	}
	type agg struct {
		line Line
		qty  Rational
		gross float64
		hasGross bool
	}
	order := []string{}
	m := map[string]*agg{}
	for _, ln := range raw {
		code := strings.TrimSpace(ln.ItemCode)
		vat := strings.TrimSpace(ln.VATRate)
		price := strings.TrimSpace(ln.UnitPriceGross)
		id := code + "|" + vat + "|" + price
		q, err := ParseQtyString(ln.Qty)
		if err != nil {
			return err
		}
		if cur, ok := m[id]; ok {
			cur.qty = AddRationals(cur.qty, q)
			if lg := strings.TrimSpace(ln.LineGross); lg != "" {
				if v, e := strconv.ParseFloat(lg, 64); e == nil {
					cur.gross += v
					cur.hasGross = true
				}
			}
			continue
		}
		cp := ln
		if strings.TrimSpace(cp.Qty) == "" {
			cp.Qty = "1"
		}
		a := &agg{line: cp, qty: q}
		if lg := strings.TrimSpace(ln.LineGross); lg != "" {
			if v, e := strconv.ParseFloat(lg, 64); e == nil {
				a.gross = v
				a.hasGross = true
			}
		}
		m[id] = a
		order = append(order, id)
	}
	out := make([]Line, 0, len(order))
	for i, id := range order {
		a := m[id]
		cp := a.line
		cp.Qty = FormatRational(a.qty)
		if a.hasGross {
			cp.LineGross = strconv.FormatFloat(a.gross, 'f', 2, 64)
		}
		cp.LineKey = LineKeyFor(cp, i)
		out = append(out, cp)
	}
	snap.SourceLines = out
	return nil
}

// EnsureSourceLines returns frozen lines or materializes once in-memory (read path).
func EnsureSourceLines(snap Snapshot) ([]Line, error) {
	if len(snap.SourceLines) > 0 {
		return snap.SourceLines, nil
	}
	cp := snap
	if err := FreezeSourceLines(&cp); err != nil {
		return nil, err
	}
	return cp.SourceLines, nil
}

// AllocationFromSplits seeds allocation from cloud splits[] (ingest).
func AllocationFromSplits(snap Snapshot) (Allocation, error) {
	src, err := EnsureSourceLines(snap)
	if err != nil {
		return Allocation{}, err
	}
	var people []AllocPerson
	for _, sp := range snap.Splits {
		sid := strings.TrimSpace(sp.ScopeID)
		if sid == "" {
			return Allocation{}, ingestErr(CodeValidationFailed, "split scope_id required")
		}
		if _, err := uuid.Parse(sid); err != nil {
			return Allocation{}, ingestErr(CodeValidationFailed, "split scope_id must be UUID")
		}
		var shares []AllocShare
		for _, ln := range sp.Lines {
			key := matchLineKey(src, ln)
			if key == "" {
				return Allocation{}, ingestErr(CodeValidationFailed, "split line not in source_lines")
			}
			qty, err := ParseQtyString(ln.Qty)
			if err != nil {
				return Allocation{}, err
			}
			shares = append(shares, AllocShare{LineKey: key, Qty: qty})
		}
		people = append(people, AllocPerson{
			ScopeID: sid,
			Name:    strings.TrimSpace(sp.Name),
			Shares:  shares,
		})
	}
	return Allocation{People: people}, nil
}

func matchLineKey(src []Line, ln Line) string {
	if k := strings.TrimSpace(ln.LineKey); k != "" {
		for _, s := range src {
			if s.LineKey == k {
				return k
			}
		}
	}
	code := strings.TrimSpace(ln.ItemCode)
	vat := strings.TrimSpace(ln.VATRate)
	price := strings.TrimSpace(ln.UnitPriceGross)
	for _, s := range src {
		if strings.TrimSpace(s.ItemCode) == code &&
			strings.TrimSpace(s.VATRate) == vat &&
			strings.TrimSpace(s.UnitPriceGross) == price {
			return s.LineKey
		}
	}
	name := strings.TrimSpace(ln.Name)
	for _, s := range src {
		if strings.TrimSpace(s.ItemCode) == code && strings.TrimSpace(s.Name) == name {
			return s.LineKey
		}
	}
	return ""
}

// ValidateAllocation checks names, UUIDs, and occupancy ≤ source_lines.
func ValidateAllocation(snap Snapshot, alloc Allocation) error {
	src, err := EnsureSourceLines(snap)
	if err != nil {
		return err
	}
	srcQty := map[string]Rational{}
	for _, ln := range src {
		q, err := ParseQtyString(ln.Qty)
		if err != nil {
			return err
		}
		srcQty[ln.LineKey] = q
	}
	seenName := map[string]bool{}
	seenScope := map[string]bool{}
	occupied := map[string]Rational{}
	for _, p := range alloc.People {
		sid := strings.TrimSpace(p.ScopeID)
		if sid == "" {
			return ingestErr(CodeValidationFailed, "allocation person scope_id required")
		}
		if _, err := uuid.Parse(sid); err != nil {
			return ingestErr(CodeValidationFailed, "allocation scope_id must be UUID")
		}
		if seenScope[sid] {
			return ingestErr(CodeValidationFailed, "duplicate scope_id in allocation")
		}
		seenScope[sid] = true
		pk := PersonKey(p.Name)
		if pk == "" {
			return ingestErr(CodeValidationFailed, "allocation person name required")
		}
		if seenName[pk] {
			return ingestErr(CodeValidationFailed, "duplicate person name")
		}
		seenName[pk] = true
		for _, sh := range p.Shares {
			if _, ok := srcQty[sh.LineKey]; !ok {
				return ingestErr(CodeValidationFailed, "share line_key not in source_lines")
			}
			q := NormalizeRational(sh.Qty)
			if q.Num <= 0 {
				return ingestErr(CodeValidationFailed, "share qty must be positive")
			}
			occupied[sh.LineKey] = AddRationals(occupied[sh.LineKey], q)
		}
	}
	for key, srcQ := range srcQty {
		if !RationalLTE(occupied[key], srcQ) {
			return ingestErr(CodeValidationFailed, fmt.Sprintf("over-allocated line %s", key))
		}
	}
	return nil
}

// RemainingPool returns unallocated qty per line_key (source − all allocation shares).
func RemainingPool(snap Snapshot, alloc Allocation) (map[string]Rational, error) {
	src, err := EnsureSourceLines(snap)
	if err != nil {
		return nil, err
	}
	out := map[string]Rational{}
	for _, ln := range src {
		q, err := ParseQtyString(ln.Qty)
		if err != nil {
			return nil, err
		}
		out[ln.LineKey] = q
	}
	for _, p := range alloc.People {
		for _, sh := range p.Shares {
			q := NormalizeRational(sh.Qty)
			cur := out[sh.LineKey]
			out[sh.LineKey] = NormalizeRational(Rational{
				Num: cur.Num*q.Den - q.Num*cur.Den,
				Den: cur.Den * q.Den,
			})
		}
	}
	return out, nil
}

// PoolEmpty reports whether every remaining qty is zero.
func PoolEmpty(snap Snapshot, alloc Allocation) (bool, error) {
	rem, err := RemainingPool(snap, alloc)
	if err != nil {
		return false, err
	}
	for _, q := range rem {
		if NormalizeRational(q).Num != 0 {
			return false, nil
		}
	}
	return true, nil
}

// IssuedScopeRef is a minimal issued scope for cleanup checks.
type IssuedScopeRef struct {
	ScopeType string
	ScopeID   string
}

// AllAllocationPeopleIssued is true when every allocation person has a signed person FT.
func AllAllocationPeopleIssued(alloc Allocation, scopes []IssuedScopeRef) bool {
	if len(alloc.People) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, sc := range scopes {
		if strings.TrimSpace(sc.ScopeType) == "person" {
			have[strings.TrimSpace(sc.ScopeID)] = true
		}
	}
	for _, p := range alloc.People {
		if !have[strings.TrimSpace(p.ScopeID)] {
			return false
		}
	}
	return true
}

// ParseAllocationJSON parses allocation_json; empty/{} → empty allocation.
func ParseAllocationJSON(raw string) (Allocation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return Allocation{}, nil
	}
	var a Allocation
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return Allocation{}, ingestErr(CodeValidationFailed, "allocation_json: "+err.Error())
	}
	return a, nil
}

// DraftPersonFromAllocation is the ONLY person SaleSnapshot builder (source_lines + allocation).
func DraftPersonFromAllocation(snap Snapshot, alloc Allocation, scopeID string) (domain.SaleSnapshot, error) {
	saleID := strings.TrimSpace(snap.SourceSaleID)
	if saleID == "" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "source_sale_id required")
	}
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "scope_id required for person issue")
	}
	if _, err := uuid.Parse(scopeID); err != nil {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("scope_id must be UUID (got %q)", scopeID))
	}
	if err := ValidateAllocation(snap, alloc); err != nil {
		return domain.SaleSnapshot{}, err
	}
	src, err := EnsureSourceLines(snap)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	byKey := map[string]Line{}
	for _, ln := range src {
		byKey[ln.LineKey] = ln
	}
	var person *AllocPerson
	for i := range alloc.People {
		if strings.TrimSpace(alloc.People[i].ScopeID) == scopeID {
			person = &alloc.People[i]
			break
		}
	}
	if person == nil {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("scope_id %q not in allocation", scopeID))
	}
	if len(person.Shares) == 0 {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "person has no shares")
	}
	var lines []Line
	for _, sh := range person.Shares {
		base, ok := byKey[sh.LineKey]
		if !ok {
			return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "missing source line")
		}
		q := NormalizeRational(sh.Qty)
		cp := base
		cp.Qty = strconv.FormatFloat(float64(q.Num)/float64(q.Den), 'f', 4, 64)
		cp.Qty = strings.TrimRight(strings.TrimRight(cp.Qty, "0"), ".")
		if cp.Qty == "" || cp.Qty == "-0" {
			cp.Qty = "0"
		}
		if lg := strings.TrimSpace(base.LineGross); lg != "" {
			srcQ, _ := ParseQtyString(base.Qty)
			if srcQ.Num > 0 {
				srcF := float64(srcQ.Num) / float64(srcQ.Den)
				shareF := float64(q.Num) / float64(q.Den)
				if v, e := strconv.ParseFloat(lg, 64); e == nil && srcF > 0 {
					cp.LineGross = strconv.FormatFloat(v*(shareF/srcF), 'f', 2, 64)
				}
			}
		} else {
			cp.LineGross = ""
		}
		lines = append(lines, cp)
	}
	outLines, gross, err := mapDraftLines(lines)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	total := strconv.FormatFloat(gross, 'f', 2, 64)
	meta := tableMeta(snap)
	if n := strings.TrimSpace(person.Name); n != "" {
		meta["split_name"] = n
	}
	return buildSaleSnapshot(saleID, "person", scopeID, outLines, total, meta), nil
}
