// Package talia provides a comprehensive toolkit for checking domain name availability
// across multiple TLDs through WHOIS lookups. It supports batch processing, concurrent
// checks, multiple output formats, and OpenAI-powered domain suggestions.
package talia

// AvailabilityReason represents the result of a domain availability check.
// It provides a standardized way to communicate why a domain is available,
// unavailable, or if an error occurred during the check.
type AvailabilityReason string

const (
	// ReasonNoMatch indicates the domain is available (no WHOIS record found).
	ReasonNoMatch AvailabilityReason = "NO_MATCH"
	// ReasonTaken indicates the domain is registered (WHOIS record exists).
	ReasonTaken AvailabilityReason = "TAKEN"
	// ReasonError indicates an error occurred during the availability check.
	ReasonError AvailabilityReason = "ERROR"
)

// DomainRecord represents a domain and its availability status.
// It is used for both input and output in non-grouped mode, where each domain
// is processed individually and its status is updated in place.
type DomainRecord struct {
	// Domain is the fully qualified domain name to check (e.g., "example.com").
	Domain string `json:"domain"`
	// Available indicates whether the domain is available for registration.
	Available bool `json:"available,omitempty"`
	// Reason provides the standardized reason code for the availability status.
	Reason AvailabilityReason `json:"reason,omitempty"`
	// Log contains the raw WHOIS response or error message when verbose mode is enabled.
	Log string `json:"log,omitempty"`
}

// GroupedDomain represents a domain in grouped output format.
// It is a more compact representation used when domains are categorized
// into available and unavailable groups.
type GroupedDomain struct {
	// Domain is the fully qualified domain name that was checked.
	Domain string `json:"domain"`
	// Reason provides the standardized reason code for the domain's status.
	Reason AvailabilityReason `json:"reason"`
	// Log contains optional WHOIS response data or error details.
	Log string `json:"log,omitempty"`
}

// GroupedData represents the output format for grouped domain checks.
// It organizes domains into two categories based on their availability status,
// making it easy to identify which domains can be registered.
type GroupedData struct {
	// Available contains all domains that are available for registration.
	Available []GroupedDomain `json:"available"`
	// Unavailable contains all domains that are already registered or had errors.
	Unavailable []GroupedDomain `json:"unavailable"`
}

// ExtendedGroupedData extends GroupedData with an additional unverified category.
// This format supports workflows where domains are suggested (e.g., by OpenAI)
// and then verified in a subsequent step.
type ExtendedGroupedData struct {
	// Available contains domains confirmed to be available for registration.
	Available []GroupedDomain `json:"available,omitempty"`
	// Unavailable contains domains that are registered or had check errors.
	Unavailable []GroupedDomain `json:"unavailable,omitempty"`
	// Unverified contains domains that have not yet been checked for availability.
	Unverified []DomainRecord `json:"unverified,omitempty"`
}
