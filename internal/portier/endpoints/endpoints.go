package endpoints

import (
	"fmt"
	"net/url"
	"strings"
)

func NormalizeBaseURL(rawBaseURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil || parsedURL.Host == "" {
		return ""
	}

	switch parsedURL.Scheme {
	case "wss":
		parsedURL.Scheme = "https"
	case "ws":
		parsedURL.Scheme = "http"
	case "https", "http":
	default:
		parsedURL.Scheme = "https"
	}

	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	parsedURL.Path = normalizeBasePath(parsedURL.Path)

	return strings.TrimSuffix(parsedURL.String(), "/")
}

func APIURL(baseURL, resourcePath string) (string, error) {
	return joinHTTPURL(baseURL, "/api"+ensureLeadingSlash(resourcePath))
}

func PublicURL(baseURL, resourcePath string) (string, error) {
	return joinHTTPURL(baseURL, "/public"+ensureLeadingSlash(resourcePath))
}

func SpiderURL(baseURL, resourcePath string) (string, error) {
	return joinHTTPURL(baseURL, "/spider"+ensureLeadingSlash(resourcePath))
}

func RelayWebsocketURL(baseURL, relayPath string) (string, error) {
	normalizedBaseURL := NormalizeBaseURL(baseURL)
	if normalizedBaseURL == "" {
		return "", fmt.Errorf("invalid base URL %q", baseURL)
	}

	parsedURL, err := url.Parse(normalizedBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}

	switch parsedURL.Scheme {
	case "https":
		parsedURL.Scheme = "wss"
	case "http":
		parsedURL.Scheme = "ws"
	default:
		parsedURL.Scheme = "wss"
	}

	parsedURL.Path = joinPath(parsedURL.Path, relayPath)
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	return parsedURL.String(), nil
}

func joinHTTPURL(baseURL, resourcePath string) (string, error) {
	normalizedBaseURL := NormalizeBaseURL(baseURL)
	if normalizedBaseURL == "" {
		return "", fmt.Errorf("invalid base URL %q", baseURL)
	}

	parsedURL, err := url.Parse(normalizedBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}

	parsedURL.Path = joinPath(parsedURL.Path, resourcePath)
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	return parsedURL.String(), nil
}

func joinPath(basePath, resourcePath string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	resourcePath = ensureLeadingSlash(resourcePath)

	if basePath == "" {
		return resourcePath
	}

	return basePath + resourcePath
}

func ensureLeadingSlash(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func normalizeBasePath(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	switch path {
	case "", "/":
		return ""
	}

	if taskRelayPath := extractTaskRelayPath(path); taskRelayPath != "" {
		return strings.TrimSuffix(path, taskRelayPath)
	}

	if strings.HasSuffix(path, "/spider") {
		return strings.TrimSuffix(path, "/spider")
	}

	if strings.HasSuffix(path, "/api") {
		return strings.TrimSuffix(path, "/api")
	}

	return path
}

func extractTaskRelayPath(path string) string {
	if index := strings.LastIndex(path, "/task-spider/"); index != -1 {
		return path[index:]
	}

	if !strings.HasSuffix(path, "/spider") {
		return ""
	}

	index := strings.LastIndex(path, "/api/tasks/")
	if index == -1 {
		return ""
	}

	return path[index:]
}
