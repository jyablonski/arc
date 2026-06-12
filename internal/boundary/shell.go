package boundary

// ShellRunner abstracts subprocess invocation and PATH probes used by cmd and internals.
// Production code typically uses github.com/jyablonski/arc/internal/shell (see shellAdapter).
//
//go:generate go tool moq -rm -out shell_runner_moq.go . ShellRunner
type ShellRunner interface {
	Run(name string, args ...string) (string, error)
	RunSudo(name string, args ...string) (string, error)
	RunInteractive(name string, args ...string) error
	CommandExists(name string) bool
}
