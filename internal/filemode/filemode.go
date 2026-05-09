package filemode

import "os"

const (
	Dir        os.FileMode = 0o755
	File       os.FileMode = 0o644
	Executable os.FileMode = 0o755
)
