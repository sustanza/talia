package talia

import (
	"errors"
	"testing"
)

type fakeWhoisClient struct {
	resp string
	err  error
}

func (f fakeWhoisClient) Lookup(domain string) (string, error) {
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
