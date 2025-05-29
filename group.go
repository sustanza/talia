// Package main contains helper functions for grouped output processing.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// mergeGrouped merges new grouped results into existing grouped data,
// deduplicating by domain.
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

// convertArrayToGrouped turns an array of DomainRecord into GroupedData.
func convertArrayToGrouped(arr []DomainRecord) GroupedData {
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

// writeGroupedFile reads an existing grouped JSON (if any), merges new data,
// and writes back to path.
func writeGroupedFile(path string, newest GroupedData) error {
	if path == "" {
		return nil
	}

	existing := GroupedData{}

	info, err := os.Stat(path)
	if err == nil && info.Size() > 0 {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read grouped file: %w", err)
		}

		if err := json.Unmarshal(raw, &existing); err != nil {
			var arr []DomainRecord
			if err2 := json.Unmarshal(raw, &arr); err2 == nil {
				existing = convertArrayToGrouped(arr)
			} else {
				return fmt.Errorf("parse grouped file: %w", err)
			}
		}
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

// replaceDomain updates an entry within the slice by domain name.
func replaceDomain(domains []DomainRecord, rec DomainRecord) {
	for i, d := range domains {
		if d.Domain == rec.Domain {
			domains[i] = rec
			return
		}
	}
}
