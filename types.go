package talia

// AvailabilityReason is a short code explaining domain availability.
type AvailabilityReason string

const (
	ReasonNoMatch AvailabilityReason = "NO_MATCH"
	ReasonTaken   AvailabilityReason = "TAKEN"
	ReasonError   AvailabilityReason = "ERROR"
)

// DomainRecord is how we parse the input array in non-grouped mode.
// "available" and "reason" are overwritten by Talia in non-grouped mode.
type DomainRecord struct {
	Domain    string             `json:"domain"`
	Available bool               `json:"available"`
	Reason    AvailabilityReason `json:"reason,omitempty"`
	Log       string             `json:"log,omitempty"`
}

// GroupedDomain is a minimal record for grouped output.
// We now include a Log field as well, so logs can be preserved in grouped mode.
type GroupedDomain struct {
	Domain string             `json:"domain"`
	Reason AvailabilityReason `json:"reason"`
	Log    string             `json:"log,omitempty"`
}

// GroupedData is the top-level object for grouped JSON. It has two arrays:
// "available" and "unavailable", each containing objects with domain + reason.
type GroupedData struct {
	Available   []GroupedDomain `json:"available"`
	Unavailable []GroupedDomain `json:"unavailable"`
}

// ExtendedGroupedData represents a grouped JSON file that may also contain
// an `unverified` list of domain records waiting to be checked.
type ExtendedGroupedData struct {
	Available   []GroupedDomain `json:"available,omitempty"`
	Unavailable []GroupedDomain `json:"unavailable,omitempty"`
	Unverified  []DomainRecord  `json:"unverified,omitempty"`
}
