package utils

import (
	"net/url"
	"strings"
)

// SanitizeForLogging removes characters that could be used for log injection attacks
func SanitizeForLogging(input string) string {
	// Remove newlines, carriage returns, and other control characters
	sanitized := strings.ReplaceAll(input, "\n", "")
	sanitized = strings.ReplaceAll(sanitized, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\t", " ")
	return sanitized
}

// SanitizeURL validates and sanitizes a URL for safe logging
func SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If URL is invalid, sanitize as regular string
		return SanitizeForLogging(rawURL)
	}

	// Return the cleaned URL string
	return parsedURL.String()
}

// SanitizeError sanitizes error messages for safe logging
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeForLogging(err.Error())
}
