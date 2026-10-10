package helpers

import (
	"golang.org/x/text/language"
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
	// Declared here rather than at each route, because this is the function
	// that reads the header: a response whose body depends on Accept-Language
	// and does not say so is one a browser or an intermediary may hand to the
	// next person in the wrong language. Nothing caches these today --
	// deploy/nginx/easydnd.conf configures no proxy_cache and the routes are
	// session-guarded -- so it is a latent bug rather than a live one, which
	// is the cheapest possible moment to fix it.
	c.Writer.Header().Add("Vary", "Accept-Language")

	if requested := c.Query(LocaleQueryParam); requested != "" {
		if locale, ok := supported(c, requested); ok {
			return locale
		}
	}
	for _, tag := range acceptedLanguages(c.GetHeader("Accept-Language")) {
		if locale, ok := supported(c, tag); ok {
			return locale
		}
	}
	return rules.DefaultLocale
}

// ContentLocales installs the catalogue's supported languages once per router.
func ContentLocales(locales []rules.Locale) gin.HandlerFunc {
	return func(c *gin.Context) { c.Set("content-locales", locales); c.Next() }
}
func supported(c *gin.Context, tag string) (rules.Locale, bool) {
	parsed, err := language.Parse(strings.TrimSpace(tag))
	if err != nil {
		return "", false
	}
	locale := parsed.String()
	available := rules.SupportedLocales()
	if value, ok := c.Get("content-locales"); ok {
		available = value.([]rules.Locale)
	}
	for locale != "" {
		for _, candidate := range available {
			if strings.EqualFold(candidate.String(), locale) {
				return candidate, true
			}
		}
		at := strings.LastIndex(locale, "-")
		if at < 0 {
			break
		}
		locale = locale[:at]
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
	tags, _, err := language.ParseAcceptLanguage(header)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, tag.String())
	}
	return out
}
