package aws

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCredentialsPath(t *testing.T) {
	t.Run("When called, it returns the correct path", func(t *testing.T) {
		usr, err := user.Current()
		if err != nil {
			t.Skipf("Skipping test: cannot get current user: %v", err)
		}

		expectedPath := filepath.Join(usr.HomeDir, ".aws", "credentials")

		path, err := GetCredentialsPath()
		require.NoError(t, err)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("When checking path format, it ends with .aws/credentials", func(t *testing.T) {
		path, err := GetCredentialsPath()
		require.NoError(t, err)

		assert.NotEmpty(t, path)

		expectedSuffix := filepath.Join(".aws", "credentials")
		assert.True(t, strings.HasSuffix(path, expectedSuffix), "path %q should end with %q", path, expectedSuffix)
	})
}
