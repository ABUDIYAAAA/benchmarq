package organizations

import (
	"testing"

	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Acme Labs":              "acme-labs",
		"  Acme -- Labs (India)": "acme-labs-india",
		"IIT Bombay 2026":        "iit-bombay-2026",
		"Été Café":               "t-caf",
		"X":                      "org-x",
		"!!!":                    "org",
	}
	for in, want := range cases {
		got := Slugify(in)
		if got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
		if _, ok := validator.ValidateStruct(struct {
			Slug string `json:"slug" validate:"slug"`
		}{got}); !ok {
			t.Errorf("Slugify(%q) produced invalid slug %q", in, got)
		}
	}
}
