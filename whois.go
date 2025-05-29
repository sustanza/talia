package talia

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// WhoisClient abstracts a WHOIS lookup mechanism.
type WhoisClient interface {
	Lookup(domain string) (string, error)
}

// NetWhoisClient performs WHOIS lookups over TCP.
type NetWhoisClient struct {
	Server string
}

// Lookup queries the configured WHOIS server for the given domain and returns
// the raw response string.
func (c NetWhoisClient) Lookup(domain string) (string, error) {
	conn, err := net.Dial("tcp", c.Server)
	if err != nil {
		return "", fmt.Errorf("failed to connect to WHOIS: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "connection close error: %v\n", cerr)
		}
	}()

	_, _ = fmt.Fprintf(conn, "%s\r\n", domain)

	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}

	data, err := io.ReadAll(conn)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty WHOIS response")
	}
	return string(data), nil
}

// CheckDomainAvailabilityWithClient queries the WHOIS client and interprets the
// response to determine availability.
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

// CheckDomainAvailability queries a WHOIS server using NetWhoisClient.
func CheckDomainAvailability(domain, server string) (bool, AvailabilityReason, string, error) {
	return CheckDomainAvailabilityWithClient(domain, NetWhoisClient{Server: server})
}
