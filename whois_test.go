package talia

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

type fakeWhoisClient struct {
	resp string
	err  error
}

func (f fakeWhoisClient) Lookup(_ string) (string, error) {
	return f.resp, f.err
}

func TestCheckDomainAvailabilityWithClient(t *testing.T) {
	cases := []struct {
		name       string
		client     WhoisClient
		wantAvail  bool
		wantReason AvailabilityReason
		wantErr    bool
	}{
		{
			name:       "available",
			client:     fakeWhoisClient{resp: "No match for example.com"},
			wantAvail:  true,
			wantReason: ReasonNoMatch,
		},
		{
			name:       "taken",
			client:     fakeWhoisClient{resp: "Domain Name: example.com"},
			wantAvail:  false,
			wantReason: ReasonTaken,
		},
		{
			name:       "error",
			client:     fakeWhoisClient{err: errors.New("dial fail")},
			wantAvail:  false,
			wantReason: ReasonError,
			wantErr:    true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			avail, reason, _, err := CheckDomainAvailabilityWithClient("example.com", tt.client)
			if avail != tt.wantAvail {
				t.Errorf("avail=%v want %v", avail, tt.wantAvail)
			}
			if reason != tt.wantReason {
				t.Errorf("reason=%s want %s", reason, tt.wantReason)
			}
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestNetWhoisClientLookupSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer helperClose(t, ln, "listener")
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			_, _ = io.Copy(io.Discard, conn)
			_, _ = io.WriteString(conn, "Hello")
			helperClose(nil, conn, "conn")
		}
	}()

	c := NetWhoisClient{Server: ln.Addr().String()}
	resp, err := c.Lookup("example.com")
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if resp != "Hello" {
		t.Fatalf("got %q want %q", resp, "Hello")
	}
}

func TestNetWhoisClientLookupDialError(t *testing.T) {
	c := NetWhoisClient{Server: "127.0.0.1:1"}
	if _, err := c.Lookup("example.com"); err == nil {
		t.Fatal("expected dial error, got nil")
	}
}

func TestNetWhoisClientLookupEmpty(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer helperClose(t, ln, "listener")
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			helperClose(nil, conn, "conn")
		}
	}()

	c := NetWhoisClient{Server: ln.Addr().String()}
	_, err = c.Lookup("example.com")
	if err == nil || !strings.Contains(err.Error(), "empty WHOIS") {
		t.Fatalf("expected empty response error, got %v", err)
	}
}
