package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestHeader(t *testing.T) {
	// Test that Header doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Header() panicked: %v", r)
		}
	}()
	Header("Test Header")
}

func TestSuccess(t *testing.T) {
	// Test that Success doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Success() panicked: %v", r)
		}
	}()
	Success("Test success")
}

func TestError(t *testing.T) {
	// Test that Error doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Error() panicked: %v", r)
		}
	}()
	Error("Test error")
}

func TestInfo(t *testing.T) {
	// Test that Info doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Info() panicked: %v", r)
		}
	}()
	Info("Test info")
}

func TestWarning(t *testing.T) {
	// Test that Warning doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Warning() panicked: %v", r)
		}
	}()
	Warning("Test warning")
}

func TestPrint(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print("Test print")

	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Test print") {
		t.Errorf("Print() output should contain message, got: %q", output)
	}
}

func TestTable(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	headers := []string{"Name", "Size"}
	rows := [][]string{
		{"package1", "100 MiB"},
		{"package2", "200 MiB"},
	}
	Table(headers, rows)

	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Name") {
		t.Errorf("Table() output should contain header 'Name', got: %q", output)
	}
	if !strings.Contains(output, "Size") {
		t.Errorf("Table() output should contain header 'Size', got: %q", output)
	}
	if !strings.Contains(output, "package1") {
		t.Errorf("Table() output should contain 'package1', got: %q", output)
	}
	if !strings.Contains(output, "package2") {
		t.Errorf("Table() output should contain 'package2', got: %q", output)
	}
}

func TestPrintKeyValue(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintKeyValue("Key", "Value")

	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Key") {
		t.Errorf("PrintKeyValue() output should contain key, got: %q", output)
	}
	if !strings.Contains(output, "Value") {
		t.Errorf("PrintKeyValue() output should contain value, got: %q", output)
	}
}
