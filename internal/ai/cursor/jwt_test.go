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

func TestSplitSessionToken_empty(t *testing.T) {
	_, _, err := splitSessionToken("")
	require.Error(t, err)

	_, _, err = splitSessionToken("   ")
	require.Error(t, err)
}

func TestSplitSessionToken_percentEncodedDoubleColon(t *testing.T) {
	uid, tok, err := splitSessionToken("user_xyz%3A%3Amy.jwt.token")
	require.NoError(t, err)
	require.Equal(t, "user_xyz", uid)
	require.Equal(t, "my.jwt.token", tok)
}

func TestSplitSessionToken_doubleColonIncomplete(t *testing.T) {
	_, _, err := splitSessionToken("nocolonshere::")
	require.Error(t, err)
}

func TestPadBase64(t *testing.T) {
	require.Equal(t, "ab==", padBase64("ab"))
	require.Equal(t, "abc=", padBase64("abc"))
	require.Equal(t, "abcd", padBase64("abcd"))
	require.Equal(t, "", padBase64(""))
}
