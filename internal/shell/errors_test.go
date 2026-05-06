package shell

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrToolNotAvailable_Error(t *testing.T) {
	e := NewErrToolNotAvailable("docker")
	require.Contains(t, e.Error(), "docker")
	require.Contains(t, e.Error(), "PATH")
	var ta *ErrToolNotAvailable
	require.True(t, errors.As(e, &ta))
	require.Equal(t, "docker", ta.Tool)
}
