package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fakeSessionProvider(name string, sessions []SessionSummary, err error) *SessionProviderMock {
	return &SessionProviderMock{
		NameFunc: func() string { return name },
		SessionsFunc: func(ctx context.Context) ([]SessionSummary, error) {
			return sessions, err
		},
	}
}

func ts(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func TestRunSessionProviders_sortsNewestFirstAndLimits(t *testing.T) {
	claude := fakeSessionProvider("claude", []SessionSummary{
		{Provider: "claude", SessionID: "c1", LastAt: ts("2026-06-01T10:00:00Z")},
		{Provider: "claude", SessionID: "c2", LastAt: ts("2026-06-03T10:00:00Z")},
	}, nil)
	codex := fakeSessionProvider("codex", []SessionSummary{
		{Provider: "codex", SessionID: "x1", LastAt: ts("2026-06-02T10:00:00Z")},
	}, nil)

	report := RunSessionProviders(context.Background(), []SessionProvider{claude, codex}, nil, SessionOptions{Limit: 2})
	require.Len(t, report.Sessions, 2)
	require.Equal(t, "c2", report.Sessions[0].SessionID) // newest
	require.Equal(t, "x1", report.Sessions[1].SessionID)
}

func TestRunSessionProviders_filtersByProviderSinceUntilAndSearch(t *testing.T) {
	claude := fakeSessionProvider("claude", []SessionSummary{
		{Provider: "claude", SessionID: "old", Project: "/home/u/arc", StartedAt: ts("2026-05-01T10:00:00Z"), LastAt: ts("2026-05-01T10:00:00Z")},
		{Provider: "claude", SessionID: "hit", Project: "/home/u/homelab", StartedAt: ts("2026-06-10T10:00:00Z"), LastAt: ts("2026-06-10T10:00:00Z")},
		{Provider: "claude", SessionID: "miss", Project: "/home/u/arc", StartedAt: ts("2026-06-10T10:00:00Z"), LastAt: ts("2026-06-10T10:00:00Z")},
	}, nil)
	codex := fakeSessionProvider("codex", []SessionSummary{
		{Provider: "codex", SessionID: "cx", Project: "/home/u/homelab", LastAt: ts("2026-06-11T10:00:00Z")},
	}, nil)

	since := ts("2026-06-01T00:00:00Z")
	report := RunSessionProviders(context.Background(), []SessionProvider{claude, codex},
		[]string{"claude"}, SessionOptions{Since: &since, Search: "homelab"})

	require.Len(t, report.Sessions, 1) // codex filtered out, "old" before since, "miss" fails search
	require.Equal(t, "hit", report.Sessions[0].SessionID)
}

func TestRunSessionProviders_isolatesProviderErrors(t *testing.T) {
	good := fakeSessionProvider("claude", []SessionSummary{
		{Provider: "claude", SessionID: "c1", LastAt: ts("2026-06-01T10:00:00Z")},
	}, nil)
	bad := fakeSessionProvider("codex", nil, errors.New("boom"))

	report := RunSessionProviders(context.Background(), []SessionProvider{good, bad}, nil, SessionOptions{})
	require.Len(t, report.Sessions, 1)
	require.NoError(t, ExitErrorIfAllSessionProvidersFailed(report)) // one provider OK

	var codexResult SessionProviderResult
	for _, p := range report.Providers {
		if p.Name == "codex" {
			codexResult = p
		}
	}
	require.False(t, codexResult.OK)
	require.Equal(t, "boom", codexResult.Error)
	require.NotEmpty(t, codexResult.Hint)
}

func TestExitErrorIfAllSessionProvidersFailed_allFail(t *testing.T) {
	report := RunSessionProviders(context.Background(), []SessionProvider{
		fakeSessionProvider("claude", nil, errors.New("a")),
		fakeSessionProvider("codex", nil, errors.New("b")),
	}, nil, SessionOptions{})
	err := ExitErrorIfAllSessionProvidersFailed(report)
	require.Error(t, err)
}
