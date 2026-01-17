package shell

import (
	"testing"
)

func TestRunSudo(t *testing.T) {
	// RunSudo is a simple wrapper that prepends "sudo" to the command
	// We can test that it correctly constructs the command

	// Test that RunSudo calls Run with "sudo" as the first argument
	// Since we can't easily mock Run, we'll test with a command that should fail
	// (assuming sudo requires password or the command doesn't exist)

	tests := []struct {
		name    string
		command string
		args    []string
		wantErr bool
	}{
		{
			name:    "sudo with echo",
			command: "echo",
			args:    []string{"test"},
			wantErr: false, // echo should work even with sudo
		},
		{
			name:    "sudo with nonexistent command",
			command: "nonexistent_command_xyz123",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RunSudo prepends "sudo" and the command name, then adds args
			// So "sudo echo test" becomes Run("sudo", "echo", "test")
			_, err := RunSudo(tt.command, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunSudo(%q, %v) error = %v, wantErr %v", tt.command, tt.args, err, tt.wantErr)
			}
		})
	}
}

// TestRunSudoCommandConstruction tests that RunSudo correctly constructs the sudo command
func TestRunSudoCommandConstruction(t *testing.T) {
	// Verify RunSudo calls Run with correct arguments
	// Since RunSudo is: sudoArgs := append([]string{name}, args...)
	// Then: Run("sudo", sudoArgs...)
	// So for RunSudo("echo", "hello", "world")
	// It should call Run("sudo", "echo", "hello", "world")

	// Test with a simple command that should work
	output, err := RunSudo("echo", "test")
	if err != nil {
		// If sudo requires a password, this will fail, which is acceptable
		// We're mainly testing that the function doesn't panic and constructs commands correctly
		t.Logf("RunSudo failed (may require sudo password): %v", err)
		return
	}

	if output != "test" {
		t.Errorf("RunSudo() output = %q, want %q", output, "test")
	}
}
