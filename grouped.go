package talia

import (
	"encoding/json"
	"fmt"
	"os"
)

// mergeGrouped merges new grouped results into existing grouped data, deduplicating by domain.
// The newest result for a domain wins. Existing entries keep their position when re-checked
// into the same bucket; new domains, and domains that change bucket, are appended in input
// order, so repeated writes produce stable output.
func mergeGrouped(existing, newest GroupedData) GroupedData {
	// latest holds the newest result per domain and which bucket it belongs in.
	type result struct {
		rec       GroupedDomain
		available bool
	}
	latest := make(map[string]result)
	for _, gd := range existing.Available {
		latest[gd.Domain] = result{gd, true}
	}
	for _, gd := range existing.Unavailable {
		latest[gd.Domain] = result{gd, false}
	}
	for _, gd := range newest.Available {
		latest[gd.Domain] = result{gd, true}
	}
	for _, gd := range newest.Unavailable {
		latest[gd.Domain] = result{gd, false}
	}

	// Emit domains in order of first appearance within their final bucket.
	out := GroupedData{}
	emitted := make(map[string]bool)
	emit := func(gd GroupedDomain, available bool) {
		r := latest[gd.Domain]
		if emitted[gd.Domain] || r.available != available {
			return
		}
		emitted[gd.Domain] = true
		if available {
			out.Available = append(out.Available, r.rec)
		} else {
			out.Unavailable = append(out.Unavailable, r.rec)
		}
	}
	for _, gd := range existing.Available {
		emit(gd, true)
	}
	for _, gd := range newest.Available {
		emit(gd, true)
	}
	for _, gd := range existing.Unavailable {
		emit(gd, false)
	}
	for _, gd := range newest.Unavailable {
		emit(gd, false)
	}
	return out
}

// ConvertArrayToGrouped turns an array of DomainRecord into GroupedData.
func ConvertArrayToGrouped(arr []DomainRecord) GroupedData {
	var gd GroupedData
	for _, rec := range arr {
		gDom := GroupedDomain{
			Domain: rec.Domain,
			Reason: rec.Reason,
			Log:    rec.Log,
		}
		if rec.Available {
			gd.Available = append(gd.Available, gDom)
		} else {
			gd.Unavailable = append(gd.Unavailable, gDom)
		}
	}
	return gd
}

// WriteGroupedFile reads an existing grouped JSON (if any), merges new data, and writes back.
// If the existing file is an array (plain DomainRecord[]), we convert it to grouped before merging.
func WriteGroupedFile(path string, newest GroupedData) error {
	if path == "" {
		return nil
	}

	existing := GroupedData{}

	info, err := os.Stat(path)
	if err == nil && info.Size() > 0 {
		if info.IsDir() {
			return fmt.Errorf("read grouped file: %s is a directory", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read grouped file: %w", err)
		}
		if err := json.Unmarshal(raw, &existing); err != nil {
			var arr []DomainRecord
			if err2 := json.Unmarshal(raw, &arr); err2 == nil {
				existing = ConvertArrayToGrouped(arr)
			} else {
				return fmt.Errorf("parse grouped file: %w", err)
			}
		}
	} else if err == nil && info.IsDir() {
		return fmt.Errorf("read grouped file: %s is a directory", path)
	}

	merged := mergeGrouped(existing, newest)
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal grouped data: %w", err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write grouped file: %w", err)
	}
	return nil
}

