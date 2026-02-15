package output

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
)

var (
	headerColor  = color.New(color.FgCyan, color.Bold)
	successColor = color.New(color.FgGreen, color.Bold)
	errorColor   = color.New(color.FgRed, color.Bold)
	infoColor    = color.New(color.FgBlue)
	warningColor = color.New(color.FgYellow)
)

// Header prints a section header
func Header(s string) {
	fmt.Println()
	headerColor.Println(s)
	fmt.Println(strings.Repeat("-", len(s)))
}

// Success prints a success message with a checkmark
func Success(s string) {
	successColor.Printf("✓ %s\n", s)
}

// Error prints an error message with an X
func Error(s string) {
	errorColor.Printf("✗ %s\n", s)
}

// Info prints an info message
func Info(s string) {
	infoColor.Printf("i %s\n", s)
}

// Warning prints a warning message
func Warning(s string) {
	warningColor.Printf("⚠ %s\n", s)
}

// Print prints a plain message
func Print(s string) {
	fmt.Println(s)
}

// Table prints a formatted table
func Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Repeat("-", len(strings.Join(headers, "\t"))))

	// Print rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	w.Flush()
}

// PrintKeyValue prints a key-value pair
func PrintKeyValue(key, value string) {
	fmt.Printf("%s:\t%s\n", key, value)
}
