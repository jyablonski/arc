package cursor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserIDFromJWT(t *testing.T) {
	const jwt = "xx.eyJzdWIiOiJvYXV0aHx1c2VyX3Rlc3RpZDEifQ.yy"
	uid, err := userIDFromJWT(jwt)
	require.NoError(t, err)
	require.Equal(t, "user_testid1", uid)
}

func TestSplitSessionToken_doubleColon(t *testing.T) {
	uid, jwt, err := splitSessionToken("user_abc::eyJ.hi")
	require.NoError(t, err)
	require.Equal(t, "user_abc", uid)
	require.Equal(t, "eyJ.hi", jwt)
}
