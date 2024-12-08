package utils

import "strings"

// GetContentType returns the content type based on the format
func GetContentType(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "application/json"
	case "csv":
		return "text/csv"
	case "xml":
		return "application/xml"
	case "html":
		return "text/html"
	case "txt":
		return "text/plain"
	case "yaml", "yml":
		return "application/x-yaml"
	case "pdf":
		return "application/pdf"
	case "zip":
		return "application/zip"
	case "png":
		return "image/png"
	case "jpeg", "jpg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	default:
		// Default to a generic binary stream content type
		return "application/octet-stream"
	}
}
