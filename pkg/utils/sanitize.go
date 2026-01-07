package utils

import (
	"net/url"
	"strings"
)

// SanitizeForLogging removes characters that could be used for log injection attacks
func SanitizeForLogging(input string) string {
	// Remove all control characters including newlines, carriage returns, tabs, and ANSI escape sequences
	var result strings.Builder
	for _, r := range input {
		// Only allow printable ASCII characters and spaces
		if r >= 32 && r <= 126 {
			result.WriteRune(r)
		}
	}
	return result.String()
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

	// Return the cleaned URL string with additional sanitization
	return SanitizeForLogging(parsedURL.String())
}

// SanitizeError sanitizes error messages for safe logging
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeForLogging(err.Error())
}
