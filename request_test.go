package blossy

import (
	"fmt"
	"testing"
)

func TestParseHash(t *testing.T) {
	tests := []struct {
		path string
		hex  string
		ext  string
	}{
		{
			path: "44f875eff24db8e87ee4057e7e5b65e50091680e6497bb8b1fbba223ec998089",
			hex:  "44f875eff24db8e87ee4057e7e5b65e50091680e6497bb8b1fbba223ec998089",
		},
		{
			path: "/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.png",
			hex:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			ext:  "png",
		},
		{
			path: "5439579437549385739845793485798347593845798347598347589357438759.pdf",
			hex:  "5439579437549385739845793485798347593845798347598347589357438759",
			ext:  "pdf",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Case=%d", i), func(t *testing.T) {
			hash, ext, err := ParseHash(test.path)
			if err != nil {
				t.Fatalf("unexpected error for path %q: %v", test.path, err)
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
