//go:build !unix

package output

import "os"

func terminalWidth(*os.File) int { return 0 }
