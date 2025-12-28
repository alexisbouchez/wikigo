package crawler

import (
	"testing"
)

func TestIsTaggedVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"standard semver", "v1.0.0", true},
		{"with minor update", "v1.2.3", true},
		{"zero version", "v0.0.0", true},
		{"large version", "v12.34.56", true},
		{"with pre-release", "v1.0.0-beta.1", true},
		{"with build metadata", "v1.0.0+build.123", true},
		{"with both", "v1.0.0-alpha.1+build", true},
		{"complex pre-release", "v2.0.0-rc.1.final", true},
		{"missing v prefix", "1.0.0", false},
		{"missing patch", "v1.0", false},
		{"missing minor", "v1", false},
		{"not a version", "latest", false},
		{"pseudo-version", "v0.0.0-20210101120000-abcdef123456", true},
		{"invalid characters", "v1.0.0@invalid", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTaggedVersion(tt.version)
			if got != tt.want {
				t.Errorf("isTaggedVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsStableVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"stable v1", "v1.0.0", true},
		{"stable v2", "v2.5.3", true},
		{"stable high version", "v10.20.30", true},
		{"unstable v0", "v0.1.0", false},
		{"unstable v0.0", "v0.0.1", false},
		{"pre-release", "v1.0.0-beta", false},
		{"pre-release alpha", "v1.0.0-alpha.1", false},
		{"release candidate", "v2.0.0-rc.1", false},
		{"with build metadata", "v1.0.0+build", true}, // build metadata doesn't affect stability
		{"pseudo-version", "v0.0.0-20210101120000-abcdef123456", false},
		{"not semver", "latest", false},
		{"missing v", "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStableVersion(tt.version)
			if got != tt.want {
				t.Errorf("isStableVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsDeprecated(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		{
			name: "starts with Deprecated",
			doc:  "Deprecated: Use NewFunc instead.",
			want: true,
		},
		{
			name: "starts with Deprecated after trim",
			doc:  "  Deprecated: This is old.",
			want: true,
		},
		{
			name: "deprecated after single newline",
			doc:  "Some text.\nDeprecated: Don't use this.",
			want: true,
		},
		{
			name: "deprecated after double newline",
			doc:  "Some text.\n\nDeprecated: Don't use this.",
			want: true,
		},
		{
			name: "not deprecated",
			doc:  "This is a normal function.",
			want: false,
		},
		{
			name: "deprecated in middle of word",
			doc:  "This is not deprecated but mentions it.",
			want: false,
		},
		{
			name: "empty doc",
			doc:  "",
			want: false,
		},
		{
			name: "only whitespace",
			doc:  "   \n\t  ",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDeprecated(tt.doc)
			if got != tt.want {
				t.Errorf("isDeprecated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRedistributable(t *testing.T) {
	tests := []struct {
		name    string
		license string
		want    bool
	}{
		{"MIT", "MIT", true},
		{"Apache 2.0", "Apache-2.0", true},
		{"BSD 2-Clause", "BSD-2-Clause", true},
		{"BSD 3-Clause", "BSD-3-Clause", true},
		{"ISC", "ISC", true},
		{"MPL 2.0", "MPL-2.0", true},
		{"Unlicense", "Unlicense", true},
		{"CC0 1.0", "CC0-1.0", true},
		{"LGPL", "LGPL", true},
		{"GPL (not redistributable)", "GPL-3.0", false},
		{"Proprietary", "Proprietary", false},
		{"Unknown", "Unknown", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRedistributable(tt.license)
			if got != tt.want {
				t.Errorf("isRedistributable(%q) = %v, want %v", tt.license, got, tt.want)
			}
		})
	}
}

func TestIdentifyLicense(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "MIT License",
			content: "MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy...",
			want:    "MIT",
		},
		{
			name:    "MIT alternative text",
			content: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software...",
			want:    "MIT",
		},
		{
			name:    "Apache 2.0",
			content: "Apache License\nVersion 2.0, January 2004\nhttp://www.apache.org/licenses/",
			want:    "Apache-2.0",
		},
		{
			name:    "BSD 3-Clause",
			content: "Redistribution and use in source and binary forms, with or without modification...\nNeither the name of the copyright holder...",
			want:    "BSD-3-Clause",
		},
		{
			name:    "BSD 3-Clause explicit",
			content: "BSD 3-Clause License\nRedistribution and use...",
			want:    "BSD-3-Clause",
		},
		{
			name:    "BSD 2-Clause",
			content: "BSD 2-Clause License\nRedistribution and use...",
			want:    "BSD-2-Clause",
		},
		{
			name:    "GPL 3.0",
			content: "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007",
			want:    "GPL-3.0",
		},
		{
			name:    "GPL 2.0",
			content: "GNU General Public License\nVersion 2, June 1991",
			want:    "GPL-2.0",
		},
		{
			name:    "MPL 2.0",
			content: "Mozilla Public License Version 2.0",
			want:    "MPL-2.0",
		},
		{
			name:    "Unlicense",
			content: "This is free and unencumbered software released into the public domain.\nThis is the Unlicense.",
			want:    "Unlicense",
		},
		{
			name:    "ISC",
			content: "ISC License\n\nPermission to use, copy, modify...",
			want:    "ISC",
		},
		{
			name:    "Unknown license",
			content: "Proprietary Software License\nAll rights reserved.",
			want:    "Unknown",
		},
		{
			name:    "Empty content",
			content: "",
			want:    "Unknown",
		},
		{
			name:    "Case insensitive",
			content: "mit LICENSE\n\nPERMISSION IS HEREBY GRANTED, FREE OF CHARGE",
			want:    "MIT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identifyLicense(tt.content)
			if got != tt.want {
				t.Errorf("identifyLicense() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModuleToRepoURL(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		want       string
	}{
		{
			name:       "GitHub module",
			modulePath: "github.com/user/repo",
			want:       "https://github.com/user/repo",
		},
		{
			name:       "GitHub with subpath",
			modulePath: "github.com/user/repo/v2",
			want:       "https://github.com/user/repo",
		},
		{
			name:       "GitLab module",
			modulePath: "gitlab.com/user/repo",
			want:       "https://gitlab.com/user/repo",
		},
		{
			name:       "GitLab with subpath",
			modulePath: "gitlab.com/group/project/subdir",
			want:       "https://gitlab.com/group/project",
		},
		{
			name:       "Bitbucket module",
			modulePath: "bitbucket.org/user/repo",
			want:       "https://bitbucket.org/user/repo",
		},
		{
			name:       "Google Source",
			modulePath: "go.googlesource.com/tools",
			want:       "https://go.googlesource.com/tools",
		},
		{
			name:       "golang.org/x package",
			modulePath: "golang.org/x/tools",
			want:       "https://go.googlesource.com/tools",
		},
		{
			name:       "golang.org/x package with subpath",
			modulePath: "golang.org/x/tools/cmd/goimports",
			want:       "https://go.googlesource.com/tools",
		},
		{
			name:       "Invalid module path (too short)",
			modulePath: "github.com",
			want:       "",
		},
		{
			name:       "Single component",
			modulePath: "module",
			want:       "",
		},
		{
			name:       "Unknown host",
			modulePath: "example.com/user/repo",
			want:       "",
		},
		{
			name:       "Empty path",
			modulePath: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := moduleToRepoURL(tt.modulePath)
			if got != tt.want {
				t.Errorf("moduleToRepoURL(%q) = %v, want %v", tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestModuleToRepoURL_EdgeCases(t *testing.T) {
	tests := []struct {
		modulePath string
		wantEmpty  bool
	}{
		{"github.com/a", true},              // Too short for GitHub
		{"gitlab.com/a", true},              // Too short for GitLab
		{"bitbucket.org/a", true},           // Too short for Bitbucket
		{"go.googlesource.com/a", false},    // Valid Google Source
		{"golang.org/x/a", false},           // Valid golang.org/x
		{"github.com/user/repo/v2/pkg", false}, // GitHub with deep path - should still return base repo
	}

	for _, tt := range tests {
		t.Run(tt.modulePath, func(t *testing.T) {
			got := moduleToRepoURL(tt.modulePath)
			isEmpty := got == ""
			if isEmpty != tt.wantEmpty {
				t.Errorf("moduleToRepoURL(%q) isEmpty=%v, want isEmpty=%v (got %q)",
					tt.modulePath, isEmpty, tt.wantEmpty, got)
			}
		})
	}
}
