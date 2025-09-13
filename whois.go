package talia

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// WhoisClient defines the interface for performing WHOIS lookups.
// This abstraction allows for easy testing and the possibility of
// implementing alternative WHOIS lookup mechanisms (e.g., REST APIs,
// cached lookups, or mock implementations for testing).
type WhoisClient interface {
    // Lookup performs a WHOIS query for the specified domain and returns
    // the raw WHOIS response. It returns an error if the lookup fails.
    // TODO(sustanza): Consider adding context-aware variant (LookupContext) to allow
    // cancellations/timeouts without relying on global deadlines.
    Lookup(domain string) (string, error)
}

// NetWhoisClient implements WhoisClient using direct TCP connections to WHOIS servers.
// It provides the standard method for querying WHOIS servers according to RFC 3912.
type NetWhoisClient struct {
    // Server specifies the WHOIS server address in "host:port" format.
    // For example: "whois.verisign-grs.com:43" for .com domains.
    Server string
}

// Lookup performs a WHOIS query by establishing a TCP connection to the configured
// WHOIS server, sending the domain query, and reading the response. The method
// handles connection management and ensures proper cleanup of resources.
// TODO(sustanza): Receiver name should be short and derived from the type (AGENTS.md Coding Style).
// Consider renaming receiver to `nwc` in a follow-up refactor.
func (c NetWhoisClient) Lookup(domain string) (string, error) {
    // TODO(sustanza): Use net.Dialer with timeouts or accept context to avoid
    // indefinite dials per best practices.
    conn, err := net.Dial("tcp", c.Server)
	if err != nil {
		return "", fmt.Errorf("failed to connect to WHOIS: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "connection close error: %v\n", cerr)
		}
	}()

    // TODO(sustanza): Check and handle the write error or document why it is safe to ignore.
    _, _ = fmt.Fprintf(conn, "%s\r\n", domain)

    if tcp, ok := conn.(*net.TCPConn); ok {
        // TODO(sustanza): Check CloseWrite error or justify ignoring it (golangci-lint errcheck).
        _ = tcp.CloseWrite()
    }

	data, err := io.ReadAll(conn)
	if err != nil && !errors.Is(err, io.EOF) {
		// Treat connection reset by peer and similar errors as empty WHOIS response
		errStr := err.Error()
		if strings.Contains(errStr, "connection reset by peer") || strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "connection closed") {
			return "", fmt.Errorf("empty WHOIS response")
		}
		return "", fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty WHOIS response")
	}
	// If the connection was closed before any data was sent, treat as empty
	if errors.Is(err, io.EOF) && len(data) == 0 {
		return "", fmt.Errorf("empty WHOIS response")
	}
	return string(data), nil
}

// CheckDomainAvailabilityWithClient performs a domain availability check using the provided
// WhoisClient implementation. It interprets the WHOIS response to determine if a domain
// is available for registration based on standard WHOIS response patterns.
// Returns:
//   - available: true if the domain is available for registration
//   - reason: the standardized reason code (NO_MATCH, TAKEN, or ERROR)
//   - logData: the raw WHOIS response or error message
//   - error: non-nil if the WHOIS lookup failed
func CheckDomainAvailabilityWithClient(domain string, client WhoisClient) (bool, AvailabilityReason, string, error) {
	resp, err := client.Lookup(domain)
	if err != nil {
		return false, ReasonError, err.Error(), err
	}
	if strings.Contains(resp, "No match for") {
		return true, ReasonNoMatch, resp, nil
	}
	return false, ReasonTaken, resp, nil
}

// CheckDomainAvailability is a convenience function that performs a domain availability
// check using the default NetWhoisClient implementation. It creates a new client with
// the specified WHOIS server and delegates to CheckDomainAvailabilityWithClient.
// This is the primary entry point for most domain availability checks.
func CheckDomainAvailability(domain, server string) (bool, AvailabilityReason, string, error) {
	return CheckDomainAvailabilityWithClient(domain, NetWhoisClient{Server: server})
}
