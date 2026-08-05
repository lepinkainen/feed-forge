package api

import (
	"net/url"
	"regexp"
	"strings"
)

// redactedPlaceholder replaces any credential found in a URL.
const redactedPlaceholder = "REDACTED"

// jwtSegmentRegex matches a JSON Web Token: three base64url runs joined by dots.
// Some feed APIs (Lemmy's /feeds/front.xml/{jwt}) carry the token as a path
// segment, so a logged URL would otherwise expose a full account session.
var jwtSegmentRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}$`)

// secretQueryKeys are query parameter names whose values are never safe to log.
var secretQueryKeys = map[string]struct{}{
	"jwt":           {},
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"secret":        {},
	"key":           {},
	"auth":          {},
	"password":      {},
	"api_key":       {},
	"apikey":        {},
}

// SanitizeURLForLog returns rawURL with credential-bearing path segments and
// query values replaced by "REDACTED", so the result is safe to put in a log
// record or an error string.
//
// The function fails closed: a URL that does not parse returns "REDACTED"
// rather than the original string.
func SanitizeURLForLog(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return redactedPlaceholder
	}

	parsed.Path = redactPath(parsed.Path)
	if parsed.User != nil {
		parsed.User = url.User(redactedPlaceholder)
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = redactQuery(parsed.RawQuery)
	}

	return parsed.String()
}

// jwtTextRegex matches a JSON Web Token anywhere in free text. It anchors on
// the "eyJ" prefix that every base64url-encoded JSON header starts with, so it
// does not mistake ordinary dotted text such as a host name for a token.
var jwtTextRegex = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}`)

// SanitizeTextForLog replaces any JSON Web Token found in text with
// "REDACTED". Use it on strings that may embed a URL, such as the message of a
// transport error.
func SanitizeTextForLog(text string) string {
	return jwtTextRegex.ReplaceAllString(text, redactedPlaceholder)
}

// sanitizedError hides credential-bearing URLs in an error's message while
// keeping the wrapped error inspectable by errors.Is and errors.As.
type sanitizedError struct {
	msg string
	err error
}

func (e *sanitizedError) Error() string { return e.msg }
func (e *sanitizedError) Unwrap() error { return e.err }

// RedactURLInError returns err with rawURL replaced by its sanitized form
// throughout the message. The transport errors returned by net/http quote the
// request URL verbatim, so an error from a request whose path carries a token
// would otherwise leak it into every log line that prints the error.
//
// The returned error still unwraps to err, so errors.Is and errors.As keep
// working.
func RedactURLInError(err error, rawURL string) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	if rawURL != "" {
		msg = strings.ReplaceAll(msg, rawURL, SanitizeURLForLog(rawURL))
	}
	msg = SanitizeTextForLog(msg)

	if msg == err.Error() {
		return err
	}
	return &sanitizedError{msg: msg, err: err}
}

// redactPath replaces every JWT-shaped segment of an URL path. A segment that
// only embeds a token rather than being one — Lemmy's /feeds/front/{jwt}.xml
// carries the ".xml" suffix inside the same segment — is handled by the
// free-text pass, which keeps the surrounding characters intact.
func redactPath(path string) string {
	if !strings.Contains(path, ".") {
		return path
	}

	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if jwtSegmentRegex.MatchString(segment) {
			segments[i] = redactedPlaceholder
			continue
		}
		segments[i] = SanitizeTextForLog(segment)
	}
	return strings.Join(segments, "/")
}

// redactQuery replaces the values of known secret query parameters. An
// unparseable query string is dropped entirely rather than logged as-is.
func redactQuery(rawQuery string) string {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return redactedPlaceholder
	}

	for key, vals := range values {
		if _, secret := secretQueryKeys[strings.ToLower(key)]; !secret {
			continue
		}
		for i := range vals {
			vals[i] = redactedPlaceholder
		}
	}
	return values.Encode()
}
