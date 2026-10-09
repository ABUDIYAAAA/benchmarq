package organizations

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"
)

const maxBaseSlugLen = 60

// Slugify converts an organization name into a URL-safe slug,
// e.g. "Acme Labs (India)" -> "acme-labs-india".
func Slugify(name string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(name) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
			if b.Len() >= maxBaseSlugLen {
				break
			}
			continue
		}
		pendingHyphen = true
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) < 3 {
		slug = strings.Trim("org-"+slug, "-")
	}
	return slug
}

// withRandomSuffix disambiguates a slug that is already taken.
func withRandomSuffix(slug string) string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return slug + "-" + hex.EncodeToString(buf)
}
