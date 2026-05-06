package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHumanizeBucketKey_sevenDaySuffix_sentenceCase(t *testing.T) {
	got := humanizeBucketKey("seven_day_foo_bar")
	require.Equal(t, "7 day (Foo Bar)", got)
}

func TestAnthropicUserMessage(t *testing.T) {
	require.Equal(t, "", anthropicUserMessage([]byte(`not json`)))
	require.Equal(t, "nope", anthropicUserMessage([]byte(`{"error":{"message":"nope"}}`)))
	require.Equal(t, "", anthropicUserMessage([]byte(`{"error":{}}`)))
}

func TestTruncate_claude(t *testing.T) {
	require.Equal(t, "x", truncate("x", 10))
	long := "abcdefghijklmnopqrstuvwxyz"
	require.Equal(t, "abcde…", truncate(long, 5))
}

func TestProvider_Name(t *testing.T) {
	require.Equal(t, "claude", (&Provider{}).Name())
}
