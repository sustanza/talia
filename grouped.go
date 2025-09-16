package talia

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

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

	sort.Slice(out.Available, func(i, j int) bool {
		return out.Available[i].Domain < out.Available[j].Domain
	})
	sort.Slice(out.Unavailable, func(i, j int) bool {
		return out.Unavailable[i].Domain < out.Unavailable[j].Domain
	})

	if out.Available == nil {
		out.Available = make([]GroupedDomain, 0)
	}
	if out.Unavailable == nil {
		out.Unavailable = make([]GroupedDomain, 0)
	}
	return out
}

// ConvertArrayToGrouped transforms a slice of DomainRecord values into grouped
// availability buckets while preserving Reason and Log details.
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

// WriteGroupedFile atomically merges grouped WHOIS results into the target path.
// Existing files are read, merged, and rewritten via a temp-file swap to avoid
// corruption; legacy array formats are upgraded automatically.
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
		raw, err := os.ReadFile(path) //nolint:gosec // user-provided JSON path
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

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("write grouped file: %w", err)
	}
	return nil
}

func replaceDomain(domains []DomainRecord, rec DomainRecord) {
	for i, d := range domains {
		if d.Domain == rec.Domain {
			domains[i] = rec
			return
		}
	}
}
