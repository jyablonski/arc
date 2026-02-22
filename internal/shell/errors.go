package shell

import "fmt"

// ErrToolNotAvailable is returned when a required CLI tool is not found in PATH.
type ErrToolNotAvailable struct {
	Tool string
}

func (e *ErrToolNotAvailable) Error() string {
	return fmt.Sprintf("%s is not available in PATH", e.Tool)
}

// NewErrToolNotAvailable creates a new ErrToolNotAvailable for the given tool name.
func NewErrToolNotAvailable(tool string) *ErrToolNotAvailable {
	return &ErrToolNotAvailable{Tool: tool}
}
