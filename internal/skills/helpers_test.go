package skills

import (
	"bytes"
	"strings"
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
		out := buf.String()
		require.Contains(t, out, "conflict backup  /backup/path")
		require.Contains(t, out, "not linked       codex › demo  arc skills sync")
		require.Contains(t, out, "name  claude  codex\ndemo    ✓       ·\n", "glyphs centre under provider headers")
		require.True(t, strings.HasSuffix(out, "⚠ 1 slot out of sync · arc skills sync\n"))
	})
	t.Run("unmanaged skills get an adoption hint", func(t *testing.T) {
		var buf bytes.Buffer
		PrintListHuman(&buf, []Provider{{Name: "claude"}}, ListResult{
			Skills:    []SkillEntry{{Name: "demo", Providers: map[string]Status{"claude": StatusOK}}},
			Unmanaged: []UnmanagedSkill{{Provider: "claude", Name: "stray", Path: "/p/stray"}},
		})
		require.Contains(t, buf.String(), "⚠ 1 unmanaged  claude › stray  arc skills add /p/stray")
		require.Contains(t, buf.String(), "✓ 1 skill in sync across 1 provider · 1 unmanaged")
	})
}
