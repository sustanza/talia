package talia

import (
    "errors"
    "fmt"
    "io"
    "context"
    "net"
    "os"
    "strings"
    "time"
)

// WhoisClient defines the interface for performing WHOIS lookups.
// This abstraction allows for easy testing and the possibility of
// implementing alternative WHOIS lookup mechanisms (e.g., REST APIs,
// cached lookups, or mock implementations for testing).
type WhoisClient interface {
    // Lookup performs a WHOIS query for the specified domain and returns
    // the raw WHOIS response. It returns an error if the lookup fails.
    Lookup(domain string) (string, error)
}

// WhoisClientContext is an optional extension that supports context-aware lookups.
// Implementations can provide cancellation/timeouts via context.
type WhoisClientContext interface {
    LookupContext(ctx context.Context, domain string) (string, error)
}

// NetWhoisClient implements WhoisClient using direct TCP connections to WHOIS servers.
// It provides the standard method for querying WHOIS servers according to RFC 3912.
type NetWhoisClient struct {
    // Server specifies the WHOIS server address in "host:port" format.
    // For example: "whois.verisign-grs.com:43" for .com domains.
    Server string
    // Timeout specifies the per-lookup timeout. If zero, a default is used.
    Timeout time.Duration
}

// Lookup performs a WHOIS query by establishing a TCP connection to the configured
// WHOIS server, sending the domain query, and reading the response. The method
// handles connection management and ensures proper cleanup of resources.
func (nwc NetWhoisClient) Lookup(domain string) (string, error) {
    // Use a Dialer with a sane timeout to avoid indefinite dials.
    tout := nwc.Timeout
    if tout <= 0 {
        tout = 10 * time.Second
    }
    d := net.Dialer{Timeout: tout}
    conn, err := d.Dial("tcp", nwc.Server)
    if err != nil {
        return "", fmt.Errorf("failed to connect to WHOIS: %w", err)
    }
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "connection close error: %v\n", cerr)
		}
	}()

    // Write the query; surface write errors.
    if _, err := fmt.Fprintf(conn, "%s\r\n", domain); err != nil {
        _ = conn.Close()
        return "", fmt.Errorf("write error: %w", err)
    }

    if tcp, ok := conn.(*net.TCPConn); ok {
        if err := tcp.CloseWrite(); err != nil {
            // Not fatal for reading response; log for visibility.
            fmt.Fprintf(os.Stderr, "closewrite error: %v\n", err)
        }
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

// LookupContext is a context-aware variant of Lookup.
func (nwc NetWhoisClient) LookupContext(ctx context.Context, domain string) (string, error) {
    tout := nwc.Timeout
    if tout <= 0 {
        tout = 10 * time.Second
    }
    d := net.Dialer{Timeout: tout}
    conn, err := d.DialContext(ctx, "tcp", nwc.Server)
    if err != nil {
        return "", fmt.Errorf("failed to connect to WHOIS: %w", err)
    }
    defer func() {
        if cerr := conn.Close(); cerr != nil {
            fmt.Fprintf(os.Stderr, "connection close error: %v\n", cerr)
        }
    }()
    if _, err := fmt.Fprintf(conn, "%s\r\n", domain); err != nil {
        _ = conn.Close()
        return "", fmt.Errorf("write error: %w", err)
    }
    if tcp, ok := conn.(*net.TCPConn); ok {
        if err := tcp.CloseWrite(); err != nil {
            fmt.Fprintf(os.Stderr, "closewrite error: %v\n", err)
        }
    }
    data, err := io.ReadAll(conn)
    if err != nil && !errors.Is(err, io.EOF) {
        errStr := err.Error()
        if strings.Contains(errStr, "connection reset by peer") || strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "connection closed") {
            return "", fmt.Errorf("empty WHOIS response")
        }
        return "", fmt.Errorf("read error: %w", err)
    }
    if len(data) == 0 {
        return "", fmt.Errorf("empty WHOIS response")
    }
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

// CheckDomainAvailabilityWithClientContext is like CheckDomainAvailabilityWithClient but
// accepts a context-aware client implementation.
func CheckDomainAvailabilityWithClientContext(ctx context.Context, domain string, client WhoisClientContext) (bool, AvailabilityReason, string, error) {
    resp, err := client.LookupContext(ctx, domain)
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
