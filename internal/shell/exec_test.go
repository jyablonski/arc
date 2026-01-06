package shell

import (
	"testing"
)

func TestCommandExists(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "ls should exist",
			command:  "ls",
			expected: true,
		},
		{
			name:     "nonexistent command should not exist",
			command:  "nonexistent_command_xyz123",
			expected: false,
		},
		{
			name:     "echo should exist",
			command:  "echo",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CommandExists(tt.command)
			if result != tt.expected {
				t.Errorf("CommandExists(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		wantErr bool
	}{
		{
			name:    "echo hello",
			command: "echo",
			args:    []string{"hello"},
			wantErr: false,
		},
		{
			name:    "true command",
			command: "true",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "false command should error",
			command: "false",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "nonexistent command should error",
			command: "nonexistent_command_xyz123",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := Run(tt.command, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run(%q, %v) error = %v, wantErr %v", tt.command, tt.args, err, tt.wantErr)
				return
			}
			if !tt.wantErr && output == "" && tt.command != "true" {
				// echo should produce output
				if tt.command == "echo" && output != "hello" {
					t.Errorf("Run(%q, %v) = %q, want %q", tt.command, tt.args, output, "hello")
				}
			}
		})
	}
}

func TestRunWithOutput(t *testing.T) {
	output, err := Run("echo", "test", "output")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	expected := "test output"
	if output != expected {
		t.Errorf("Run() = %q, want %q", output, expected)
	}
}

func TestRunWithWhitespace(t *testing.T) {
	output, err := Run("echo", "  hello  ")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// TrimSpace removes leading/trailing whitespace from output
	// echo prints the argument, but TrimSpace will trim it
	if output != "hello" {
		t.Errorf("Run() = %q, want %q", output, "hello")
	}
}
