package blossy

import (
	"fmt"
	"testing"
)

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		hostname string
		isValid  bool
	}{

		// invalid: empty
		{"", false},

		// invalid: includes scheme
		{"https://example.com", false},
		{"http://example.com", false},
		{"ftp://example.com", false},

		// invalid: includes path
		{"example.com/", false},
		{"example.com/blossom", false},
		{"example.com/path/to/resource", false},

		// invalid: includes query or fragment
		{"example.com?query=1", false},
		{"example.com#fragment", false},

		// invalid: scheme + path
		{"https://example.com/blossom", false},

		// valid hostnames
		{"example.com", true},
		{"cdn.example.com", true},
		{"blossom.example.com", true},
		{"sub.domain.example.com", true},
		{"example.com:8080", true},
		{"localhost:3000", true},
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.1:3000", true},
		{"my-server.example.com", true},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Case=%d_%s", i, test.hostname), func(t *testing.T) {
			err := validateHostname(test.hostname)
			if test.isValid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", test.hostname, err)
			}
			if !test.isValid && err == nil {
				t.Errorf("expected %q to be invalid, but got no error", test.hostname)
			}
		})
	}
}
