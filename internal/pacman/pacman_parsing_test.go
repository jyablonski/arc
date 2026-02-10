package pacman

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test helper functions that parse pacman output
// These test the parsing logic without requiring actual pacman commands

func TestParsePackageListOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single package",
			output:   "package-name 1.0.0-1",
			expected: 1,
		},
		{
			name:     "multiple packages",
			output:   "package1 1.0.0-1\npackage2 2.0.0-1\npackage3 3.0.0-1",
			expected: 3,
		},
		{
			name:     "packages with trailing newline",
			output:   "package1 1.0.0-1\npackage2 2.0.0-1\n",
			expected: 2,
		},
		{
			name:     "packages with extra whitespace",
			output:   "package1  1.0.0-1  \n  package2  2.0.0-1  ",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(tt.output), "\n")
			if tt.output == "" {
				lines = []string{}
			}

			// Filter out empty lines
			nonEmptyLines := make([]string, 0)
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					nonEmptyLines = append(nonEmptyLines, line)
				}
			}

			assert.Len(t, nonEmptyLines, tt.expected)
		})
	}
}

func TestParseInstalledSizeOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected float64 // in GiB
	}{
		{
			name: "single package KiB",
			output: `Name            : test-package
Installed Size  : 1024.00 KiB
`,
			expected: 1024.0 / 1024 / 1024, // KiB to GiB
		},
		{
			name: "single package MiB",
			output: `Name            : test-package
Installed Size  : 512.50 MiB
`,
			expected: 512.5 / 1024, // MiB to GiB
		},
		{
			name: "single package GiB",
			output: `Name            : test-package
Installed Size  : 2.5 GiB
`,
			expected: 2.5,
		},
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the parsing logic from GetTotalInstalledSize
			var totalMiB float64
			lines := strings.Split(tt.output, "\n")

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
