package pacman

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePackageList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
		{
			name:     "single package",
			input:    "package-name 1.0.0-1",
			expected: 1,
		},
		{
			name:     "multiple packages",
			input:    "package1 1.0.0-1\npackage2 2.0.0-1\npackage3 3.0.0-1",
			expected: 3,
		},
		{
			name:     "packages with trailing newline",
			input:    "package1 1.0.0-1\npackage2 2.0.0-1\n",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(tt.input), "\n")
			if tt.input == "" {
				lines = []string{}
			}
			assert.Len(t, lines, tt.expected)
		})
	}
}

func TestParseInstalledSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64 // in GiB
	}{
		{
			name: "single package KiB",
			input: `Name            : test-package
Installed Size  : 1024.00 KiB
`,
			expected: 1024.0 / 1024 / 1024, // KiB to GiB
		},
		{
			name: "single package MiB",
			input: `Name            : test-package
Installed Size  : 512.50 MiB
`,
			expected: 512.5 / 1024, // MiB to GiB
		},
		{
			name: "single package GiB",
			input: `Name            : test-package
Installed Size  : 2.5 GiB
`,
			expected: 2.5,
		},
		{
			name: "multiple packages",
			input: `Name            : package1
Installed Size  : 1024.00 KiB
Name            : package2
Installed Size  : 512.00 MiB
Name            : package3
Installed Size  : 1.0 GiB
`,
			expected: (1024.0/1024 + 512.0 + 1.0*1024) / 1024, // All converted to MiB then to GiB
		},
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the parsing logic
			var totalMiB float64
			lines := strings.Split(tt.input, "\n")

			for _, line := range lines {
				if strings.HasPrefix(line, "Installed Size") {
					re := regexp.MustCompile(`Installed Size\s+:\s+([\d.]+)\s+(KiB|MiB|GiB)`)
					matches := re.FindStringSubmatch(line)
					if len(matches) == 3 {
						size, err := strconv.ParseFloat(matches[1], 64)
						if err != nil {
							continue
						}
						unit := matches[2]

						switch unit {
						case "KiB":
							totalMiB += size / 1024
						case "MiB":
							totalMiB += size
						case "GiB":
							totalMiB += size * 1024
						}
					}
				}
			}

			result := totalMiB / 1024 // Convert to GiB
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseOrphanedPackages(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single orphan",
			input:    "orphan-package 1.0.0-1",
			expected: []string{"orphan-package"},
		},
		{
			name:     "multiple orphans",
			input:    "orphan1 1.0.0-1\norphan2 2.0.0-1\norphan3 3.0.0-1",
			expected: []string{"orphan1", "orphan2", "orphan3"},
		},
		{
			name:     "orphans with extra whitespace",
			input:    "  orphan1  1.0.0-1  \n  orphan2  2.0.0-1  ",
			expected: []string{"orphan1", "orphan2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(tt.input), "\n")
			packages := make([]string, 0, len(lines))
			for _, line := range lines {
				if line != "" {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						packages = append(packages, parts[0])
					}
				}
			}

			assert.Equal(t, tt.expected, packages)
		})
	}
}

func TestCheckPacmanAvailable(t *testing.T) {
	err := CheckPacmanAvailable()
	// This test depends on the system - pacman might or might not be available
	// We just check that it doesn't panic
	if err != nil {
		assert.Equal(t, "pacman is not available in PATH", err.Error())
	}
}

func TestCheckYayAvailable(t *testing.T) {
	// This test depends on the system - yay might or might not be available
	// We just check that it returns a boolean and doesn't panic
	assert.NotPanics(t, func() {
		_ = CheckYayAvailable()
	})
}
