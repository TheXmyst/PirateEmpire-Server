package client

import "testing"

func TestCheckSecureURL(t *testing.T) {
	ok := []string{"https://seadogs.duckdns.org", "https://seadogs.duckdns.org/version", "http://localhost:8080", "http://127.0.0.1:8080", "http://0.0.0.0:8080"}
	bad := []string{"http://seadogs.duckdns.org", "http://169.58.39.158:8080", "ftp://example.com", "::not a url"}
	for _, u := range ok {
		if err := CheckSecureURL(u); err != nil {
			t.Errorf("%q should be accepted: %v", u, err)
		}
	}
	for _, u := range bad {
		if CheckSecureURL(u) == nil {
			t.Errorf("%q should be rejected", u)
		}
	}
}
