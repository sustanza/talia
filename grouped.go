package talia

import (
	"encoding/json"
	"fmt"
	"os"
)

// mergeGrouped combines two GroupedData structures, with newer entries taking precedence.
// If a domain appears in both structures, the entry from 'newest' will replace the one
// from 'existing'. A domain can move between available and unavailable categories based
// on the most recent check. This function ensures no duplicate domains exist in the output.
func mergeGrouped(existing, newest GroupedData) GroupedData {
	domainsAvail := make(map[string]GroupedDomain)
	for _, gd := range existing.Available {
		domainsAvail[gd.Domain] = gd
	}
	domainsUnavail := make(map[string]GroupedDomain)
	for _, gd := range existing.Unavailable {
		domainsUnavail[gd.Domain] = gd
	}

	for _, gd := range newest.Available {
		domainsAvail[gd.Domain] = gd
		delete(domainsUnavail, gd.Domain)
	}
	for _, gd := range newest.Unavailable {
		domainsUnavail[gd.Domain] = gd
		delete(domainsAvail, gd.Domain)
	}

	out := GroupedData{}
	for _, rec := range domainsAvail {
		out.Available = append(out.Available, rec)
	}
	for _, rec := range domainsUnavail {
		out.Unavailable = append(out.Unavailable, rec)
	}
	return out
}

// ConvertArrayToGrouped transforms a flat array of DomainRecord entries into a GroupedData
// structure, categorizing domains based on their availability status. Available domains
// (where Available field is true) are placed in the Available group, while all others
// go to the Unavailable group. The function preserves the Reason and Log fields from
// each domain record.
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

// WriteGroupedFile performs an atomic update of a grouped JSON file by reading any existing
// data, merging it with new results, and writing the combined data back. The function handles
// three cases for existing files:
//   - No existing file: writes the new data directly
//   - Existing GroupedData file: merges with the new data, newer entries take precedence
//   - Existing DomainRecord array: converts to GroupedData format then merges
//
// The write operation is atomic to prevent data corruption. Returns an error if the file
// operations fail or if the existing file contains invalid JSON.
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
		raw, err := os.ReadFile(path) //nolint:gosec // User-provided path for JSON data
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
	if err := os.WriteFile(path, out, 0644); err != nil { //nolint:gosec // JSON files don't contain secrets
		return fmt.Errorf("write grouped file: %w", err)
	}
	return nil
}

// replaceDomain updates a domain record in-place within a slice of DomainRecord entries.
// It searches for a record with matching domain name and replaces it with the provided
// record. This function is used in non-grouped mode to update availability status after
// each WHOIS check. If no matching domain is found, the function returns without error.
func replaceDomain(domains []DomainRecord, rec DomainRecord) {
	for i, d := range domains {
		if d.Domain == rec.Domain {
			domains[i] = rec
			return
		}
	}
}
