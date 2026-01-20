package utils

import (
	"net/url"
	"strings"
	"unicode"

	ctrl "sigs.k8s.io/controller-runtime"
)

// SanitizeForLogging removes characters that could be used for log injection attacks
func SanitizeForLogging(input string) string {
	var result strings.Builder
	result.Grow(len(input))
	for _, r := range input {
		if unicode.IsPrint(r) && r != '\n' && r != '\r' && r != '\x1b' {
			if _, err := result.WriteRune(r); err != nil {
				return result.String()
			}
		}
	}
	return result.String()
}

// SanitizeURL validates and sanitizes a URL for safe logging
func SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		ctrl.Log.Error(err, "Failed to parse URL", "rawURL", SanitizeForLogging(rawURL))
		return SanitizeForLogging(rawURL)
	}

	parsedURL.User = nil
	parsedURL.RawQuery = ""
	return parsedURL.String()
}

// SanitizeError sanitizes error messages for safe logging
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeForLogging(err.Error())
}
