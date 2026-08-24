package helpers

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// LocaleQueryParam is the query parameter that overrides content negotiation.
const LocaleQueryParam = "locale"

// Locale picks the locale for a request.
//
// The order is explicit override, then Accept-Language, then the default. The
// query parameter exists because a language switcher is a thing a user
// clicks, and rewriting a browser's Accept-Language header to honour it is
// not something a browser lets a page do.
//
// An unsupported locale falls back rather than failing. Content negotiation
// is a preference, not a request: a browser sending "Accept-Language: fr" is
// saying what it would like, and answering 406 to that is a worse experience
// than answering in English.
func Locale(c *gin.Context) rules.Locale {
	if requested := c.Query(LocaleQueryParam); requested != "" {
		if locale, ok := supported(requested); ok {
			return locale
		}
	}
	for _, tag := range acceptedLanguages(c.GetHeader("Accept-Language")) {
		if locale, ok := supported(tag); ok {
			return locale
		}
	}
	return rules.DefaultLocale
}

func supported(tag string) (rules.Locale, bool) {
	locale := rules.Locale(strings.ToLower(strings.TrimSpace(tag)))
	if locale.IsSupported() {
		return locale, true
	}
	return "", false
}

// acceptedLanguages parses an Accept-Language header into language tags in
// the order the client prefers them.
//
// Quality values are read but not sorted on: the header is conventionally
// written in preference order already, and a client that writes it otherwise
// is asking for a subtlety this application has no way to reward -- there are
// two locales.
func acceptedLanguages(header string) []string {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		tag, _, _ := strings.Cut(part, ";")
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "*" {
			continue
		}
		out = append(out, tag)
		// "ru-RU" should match the "ru" bundle.
		if base, _, found := strings.Cut(tag, "-"); found {
			out = append(out, base)
		}
	}
	return out
}
