// Package main contains data structures for Talia domain processing.
package main

// AvailabilityReason is a short code explaining domain availability.
type AvailabilityReason string

const (
	ReasonNoMatch AvailabilityReason = "NO_MATCH"
	ReasonTaken   AvailabilityReason = "TAKEN"
	ReasonError   AvailabilityReason = "ERROR"
)

// DomainRecord represents a domain entry when using non-grouped JSON input.
type DomainRecord struct {
	Domain    string             `json:"domain"`
	Available bool               `json:"available"`
	Reason    AvailabilityReason `json:"reason,omitempty"`
	Log       string             `json:"log,omitempty"`
}

// GroupedDomain is a minimal record for grouped output. Logs may be included.
type GroupedDomain struct {
	Domain string             `json:"domain"`
	Reason AvailabilityReason `json:"reason"`
	Log    string             `json:"log,omitempty"`
}

// GroupedData is the top-level object for grouped JSON output.
type GroupedData struct {
	Available   []GroupedDomain `json:"available"`
	Unavailable []GroupedDomain `json:"unavailable"`
}

// ExtendedGroupedData is used when the input grouped file also contains
// an "unverified" list that should be processed.
type ExtendedGroupedData struct {
	Available   []GroupedDomain `json:"available,omitempty"`
	Unavailable []GroupedDomain `json:"unavailable,omitempty"`
	Unverified  []DomainRecord  `json:"unverified,omitempty"`
}
