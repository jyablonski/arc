package filemode

import "os"

const (
	Dir        os.FileMode = 0o755
	File       os.FileMode = 0o644
	Executable os.FileMode = 0o755

	// Private is for files that may carry credentials or session state, such
	// as the AI providers' own config files. It matches the mode those tools
	// use themselves.
	Private os.FileMode = 0o600
)
