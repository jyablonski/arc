package platform

import "runtime"

type OS int

const (
	Unknown OS = iota
	Linux
	Darwin
)

func (o OS) String() string {
	switch o {
	case Linux:
		return "linux"
	case Darwin:
		return "darwin"
	default:
		return "unknown"
	}
}

func Detect() OS {
	switch runtime.GOOS {
	case "linux":
		return Linux
	case "darwin":
		return Darwin
	default:
		return Unknown
	}
}
