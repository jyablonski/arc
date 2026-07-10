package skills

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContains(t *testing.T) {
	require.True(t, contains([]string{"a", "b"}, "a"))
	require.False(t, contains([]string{"a"}, "z"))
}

func TestPrintListHuman(t *testing.T) {
	t.Run("empty skills", func(t *testing.T) {
		var buf bytes.Buffer
		PrintListHuman(&buf, nil, ListResult{})
		require.Contains(t, buf.String(), "no skills found")
	})
	t.Run("conflicts written to writer", func(t *testing.T) {
		var buf bytes.Buffer
		providers := []Provider{{Name: "claude"}, {Name: "codex"}}
		PrintListHuman(&buf, providers, ListResult{
			Skills: []SkillEntry{
				{Name: "demo", CanonicalPath: "/canon", Providers: map[string]Status{"claude": StatusOK, "codex": StatusMissing}},
			},
			Conflicts: []ConflictBackup{{Provider: "claude", Path: "/backup/path"}},
		})
		require.Contains(t, buf.String(), "Conflict backups")
		require.Contains(t, buf.String(), "/backup/path")
	})
}
