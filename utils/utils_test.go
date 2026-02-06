package utils

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestParseHashExt(t *testing.T) {
	tests := []struct {
		path    string
		hex     string
		ext     string
		isValid bool
	}{
		// valid
		{"44f875eff24db8e87ee4057e7e5b65e50091680e6497bb8b1fbba223ec998089", "44f875eff24db8e87ee4057e7e5b65e50091680e6497bb8b1fbba223ec998089", "", true},
		{"/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.png", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "png", true},
		{"5439579437549385739845793485798347593845798347598347589357438759.pdf", "5439579437549385739845793485798347593845798347598347589357438759", "pdf", true},
		{"/aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd.tar.gz", "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd", "tar.gz", true},

		// invalid: empty or bare slash
		{"", "", "", false},
		{"/", "", "", false},

		// invalid: too short
		{"tooshort.png", "", "", false},

		// invalid: bad hex characters
		{"xyz_invalid_hex_0123456789abcdef0123456789abcdef0123456789abcdef", "", "", false},

		// invalid: wrong length
		{"/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde", "", "", false},  // 63 chars
		{"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0", "", "", false}, // 65 chars
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Case=%d", i), func(t *testing.T) {
			hash, ext, err := ParseHashExt(test.path)
			if test.isValid && err != nil {
				t.Fatalf("unexpected error for path %q: %v", test.path, err)
			}
			if !test.isValid {
				if err == nil {
					t.Fatalf("expected error for path %q, but got nil", test.path)
				}
				return
			}

			if hash.Hex() != test.hex {
				t.Errorf("expected hash %v, got %v", test.hex, hash.Hex())
			}
			if ext != test.ext {
				t.Errorf("expected ext %v, got %v", test.ext, ext)
			}
		})
	}
}

func TestValidateBlossomURL(t *testing.T) {
	tests := []struct {
		rawURL  string
		isValid bool
	}{
		// valid blossom URLs
		{"https://cdn.example.com/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
		{"https://cdn.example.com/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.png", true},
		{"http://localhost:3000/aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd.jpg", true},

		// invalid: no hash in path
		{"https://cdn.example.com/", false},
		{"https://cdn.example.com/tooshort", false},

		// invalid: hash too short
		{"https://cdn.example.com/abcdef.png", false},

		// invalid: bad hex characters
		{"https://cdn.example.com/xyz_invalid_hex_0123456789abcdef0123456789abcdef0123456789abcdef", false},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Case=%d", i), func(t *testing.T) {
			u, err := url.Parse(test.rawURL)
			if err != nil {
				t.Fatalf("failed to parse test URL %q: %v", test.rawURL, err)
			}

			err = ValidateBlossomURL(u)
			if test.isValid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", test.rawURL, err)
			}
			if !test.isValid && err == nil {
				t.Errorf("expected %q to be invalid, but got no error", test.rawURL)
			}
		})
	}
}

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
			err := ValidateHostname(test.hostname)
			if test.isValid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", test.hostname, err)
			}
			if !test.isValid && err == nil {
				t.Errorf("expected %q to be invalid, but got no error", test.hostname)
			}
		})
	}
}

func TestReadNoMore(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		limit   int
		isValid bool
	}{
		// valid: well under limit
		{"small body", "hello", 100, true},

		// valid: empty body
		{"empty body", "", 10, true},

		// valid: body is exactly limit bytes (inclusive)
		{"exactly limit", strings.Repeat("a", 10), 10, true},

		// invalid: body is limit+1 bytes
		{"limit+1", strings.Repeat("a", 11), 10, false},

		// invalid: body well exceeds limit
		{"exceeds limit", strings.Repeat("a", 20), 10, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := strings.NewReader(test.body)
			data, err := ReadNoMore(reader, test.limit)

			if test.isValid && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !test.isValid && err == nil {
				t.Fatalf("expected error for body of len %d with limit %d, got nil", len(test.body), test.limit)
			}
			if test.isValid && string(data) != test.body {
				t.Errorf("expected body %q, got %q", test.body, string(data))
			}
		})
	}
}

func TestDecodeBase64(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		isValid bool
	}{
		{"padded", "dGhpcy9pcy9hL3Rlc3Q=", "this/is/a/test", true},
		{"raw", "dGhpcy9pcy9hL3Rlc3Q", "this/is/a/test", true},

		// payload that produces +/ in standard, -_ in URL-safe
		{"std with +/", "+/+/", "\xfb\xff\xbf", true},
		{"url with -_", "-_-_", "\xfb\xff\xbf", true},
		{"std padded with +/", "+/+/+w==", "\xfb\xff\xbf\xfb", true},
		{"url padded with -_", "-_-_-w==", "\xfb\xff\xbf\xfb", true},

		// neutral payload: no distinguishing characters
		{"neutral padded", "AA==", "\x00", true},
		{"neutral raw", "AA", "\x00", true},

		// invalid
		{"ambiguous", "+/-_", "", false},
		{"not base64", "!!", "", false},
		{"bad padding", "AQID=", "", false},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d_%s", i, test.name), func(t *testing.T) {
			got, err := DecodeBase64(test.input)

			if !test.isValid {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", test.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %q: %v", test.input, err)
			}
			if string(got) != test.want {
				t.Errorf("expected %x, got %x", test.want, got)
			}
		})
	}
}
