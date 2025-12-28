package util

import (
	"os"
	"path/filepath"
	"strings"
)

// IsDeprecated checks if documentation text indicates deprecation
func IsDeprecated(docText string) bool {
	docText = strings.TrimSpace(docText)
	if strings.HasPrefix(docText, "Deprecated:") {
		return true
	}
	return strings.Contains(docText, "\nDeprecated:") || strings.Contains(docText, "\n\nDeprecated:")
}

// IsRedistributable checks if a license allows redistribution
func IsRedistributable(license string) bool {
	redistributable := map[string]bool{
		"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
		"ISC": true, "MPL-2.0": true, "Unlicense": true, "CC0-1.0": true, "LGPL": true,
	}
	return redistributable[license]
}

// DetectLicense detects the license type and text from a directory
func DetectLicense(dir string) (licenseType string, licenseText string) {
	licenseFiles := []string{
		"LICENSE", "LICENSE.txt", "LICENSE.md", "LICENCE", "LICENCE.txt",
		"COPYING", "COPYING.txt",
	}

	for _, name := range licenseFiles {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(content)
		return IdentifyLicense(text), text
	}
	return "", ""
}

// IdentifyLicense identifies the license type from license text
func IdentifyLicense(content string) string {
	content = strings.ToLower(content)
	switch {
	case strings.Contains(content, "apache license") && strings.Contains(content, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(content, "mit license") || strings.Contains(content, "permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(content, "bsd 3-clause") || (strings.Contains(content, "redistribution and use") && strings.Contains(content, "neither the name")):
		return "BSD-3-Clause"
	case strings.Contains(content, "bsd 2-clause"):
		return "BSD-2-Clause"
	case strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 3"):
		return "GPL-3.0"
	case strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 2"):
		return "GPL-2.0"
	case strings.Contains(content, "mozilla public license") && strings.Contains(content, "2.0"):
		return "MPL-2.0"
	case strings.Contains(content, "unlicense"):
		return "Unlicense"
	case strings.Contains(content, "isc license"):
		return "ISC"
	}
	return "Unknown"
}

// ModuleToRepoURL converts a Go module path to a repository URL
func ModuleToRepoURL(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 2 {
		return ""
	}
	host := parts[0]
	switch {
	case host == "github.com" && len(parts) >= 3:
		return "https://github.com/" + parts[1] + "/" + parts[2]
	case host == "gitlab.com" && len(parts) >= 3:
		return "https://gitlab.com/" + parts[1] + "/" + parts[2]
	case host == "bitbucket.org" && len(parts) >= 3:
		return "https://bitbucket.org/" + parts[1] + "/" + parts[2]
	case strings.HasPrefix(host, "go.googlesource.com"):
		return "https://go.googlesource.com/" + parts[1]
	case host == "golang.org" && len(parts) >= 3 && parts[1] == "x":
		return "https://go.googlesource.com/" + parts[2]
	}
	return ""
}
