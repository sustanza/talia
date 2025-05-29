// Package main implements WHOIS querying utilities.
package main

import (
	"fmt"
	"io"
	"net"
	"strings"
)

// checkDomainAvailability queries the WHOIS server for a single domain.
// It returns availability, reason, raw WHOIS response, and an error if any.
func checkDomainAvailability(domain, server string) (bool, AvailabilityReason, string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		logMsg := fmt.Sprintf("failed to connect to WHOIS: %v", err)
		return false, ReasonError, logMsg, fmt.Errorf("failed to connect to WHOIS: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\r\n", domain)
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	data, err := io.ReadAll(conn)
	if err != nil && err != io.EOF {
		readMsg := fmt.Sprintf("read error: %v", err)
		return false, ReasonError, readMsg, fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return false, ReasonError, "empty WHOIS response", fmt.Errorf("empty WHOIS response")
	}

	response := string(data)
	if strings.Contains(response, "No match for") {
		return true, ReasonNoMatch, response, nil
	}
	return false, ReasonTaken, response, nil
}
