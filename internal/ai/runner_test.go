package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubProv struct {
	name string
	rep  UsageReport
	err  error
}

func (s *stubProv) Name() string { return s.name }

func (s *stubProv) Usage(ctx context.Context) (UsageReport, error) {
	return s.rep, s.err
}

func TestRunProviders_noFilter(t *testing.T) {
	p := []Provider{
		&stubProv{name: "a", rep: UsageReport{Windows: []UsageWindow{{Label: "w", PercentUsed: 5}}}},
		&stubProv{name: "b", err: errors.New("boom")},
	}
	agg := RunProviders(context.Background(), p, nil)
	require.Len(t, agg.Providers, 2)
	require.True(t, agg.Providers[0].OK)
	require.False(t, agg.Providers[1].OK)
	require.Contains(t, agg.Providers[1].Error, "boom")
}

func TestRunProviders_filter(t *testing.T) {
	p := []Provider{
		&stubProv{name: "claude"},
		&stubProv{name: "codex"},
	}
	agg := RunProviders(context.Background(), p, []string{"codex"})
	require.Len(t, agg.Providers, 1)
	require.Equal(t, "codex", agg.Providers[0].Name)
}

func TestCombineErrors(t *testing.T) {
	err := CombineErrors(AggregateReport{Providers: []ProviderResult{
		{Name: "a", OK: false, Error: "e1"},
		{Name: "b", OK: false, Error: "e2"},
	}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "e1")
}

func TestHintFor(t *testing.T) {
	require.Contains(t, hintFor("claude", nil), "credentials")
	require.Contains(t, hintFor("codex", nil), "Codex")
	require.Contains(t, hintFor("cursor", nil), "state.vscdb")
	require.Equal(t, "", hintFor("other", nil))
}
