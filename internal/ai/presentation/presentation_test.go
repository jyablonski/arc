package presentation

import (
	"io"
	"os"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestCompactDurationRemain(t *testing.T) {
	require.Equal(t, "30m", compactDurationRemain(30*time.Minute))
	require.Equal(t, "5h 0m", compactDurationRemain(5*time.Hour))
	require.Equal(t, "5h 30m", compactDurationRemain(5*time.Hour+30*time.Minute))
	require.Equal(t, "2d 3h", compactDurationRemain(51*time.Hour))
	require.Equal(t, "12d", compactDurationRemain(12*24*time.Hour))
}

func TestPctRemainingForDisplay(t *testing.T) {
	require.Equal(t, -1.0, pctRemainingForDisplay(-1))
	require.InDelta(t, 100, pctRemainingForDisplay(0), 0.001)
	require.InDelta(t, 72, pctRemainingForDisplay(28), 0.001)
	require.InDelta(t, 24.6, pctRemainingForDisplay(75.4), 0.05)
	require.InDelta(t, 0, pctRemainingForDisplay(100), 0.001)
	require.InDelta(t, 0, pctRemainingForDisplay(150), 0.001)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	var buf syncBuffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
	}()

	fn()
	require.NoError(t, w.Close())
	os.Stdout = oldStdout
	wg.Wait()
	return buf.String()
}

type syncBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, p...)
	return len(p), nil
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.buf)
}

func TestPrintAggregate_successAndFormatting(t *testing.T) {
	now := time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC)
	reset := now.Add(3 * time.Hour)
	out := captureStdout(t, func() {
		PrintAggregate(ai.AggregateReport{
			FetchedAt: now,
			Providers: []ai.ProviderResult{
				{
					Name: "claude",
					OK:   true,
					Report: ai.UsageReport{
						Windows: []ai.UsageWindow{{Label: "5 hour", PercentUsed: 33.3, ResetsAt: &reset}},
					},
				},
			},
		})
	})
	require.Contains(t, out, "Claude")
	require.Contains(t, out, "5 hour")
	require.Contains(t, out, "window")
	require.Contains(t, out, "% left")
	require.Contains(t, out, "●")
}

func TestPrintAggregate_providerErrors(t *testing.T) {
	out := captureStdout(t, func() {
		PrintAggregate(ai.AggregateReport{
			Providers: []ai.ProviderResult{
				{Name: "codex", OK: false, Error: "offline", Hint: "install codex"},
			},
		})
	})
	require.Contains(t, out, "offline")
	require.Contains(t, out, "install codex")
	require.Contains(t, out, "Codex")
}

func TestPrintAggregate_emptyWindowsMessage(t *testing.T) {
	out := captureStdout(t, func() {
		PrintAggregate(ai.AggregateReport{
			Providers: []ai.ProviderResult{
				{Name: "cursor", OK: true, Report: ai.UsageReport{}},
			},
		})
	})
	require.Contains(t, out, "no usage windows returned")
	require.Contains(t, out, "Cursor")
}

func TestCapitalizeAndPadRunes_unexportedViaBehavior(t *testing.T) {
	require.Equal(t, "", capitalize(""))
	require.Equal(t, "Hello", capitalize("hello"))
	require.Equal(t, "abc…", padRunes("abcdefghijklmnopqrstuvwxyz01234567890xx", 4))
	require.Equal(t, "", padRunes("ab", 1))
	padded := padRunes("hi", 10)
	require.Equal(t, 10, utf8.RuneCountInString(padded))
	require.True(t, padded[:2] == "hi")
}

func TestAlignPercentCell_renderRemainBar_formatRemainPercentHuman(t *testing.T) {
	accent := providerAccent("codex")
	require.Contains(t, alignPercentCell("99.9%"), "99.9%")
	require.Contains(t, renderRemainBar(accent, 50, 10), "●")
	require.Contains(t, renderRemainBar(accent, -1, 10), "·")

	green := color.New(color.FgGreen)
	require.Contains(t, formatRemainPercentHuman(25, green), "75.0%")
	require.Contains(t, formatRemainPercentHuman(-1, green), "—")
}

func TestFormatResetHuman(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, "-", formatResetHuman(nil, now))
	require.Equal(t, "now", formatResetHuman(ptrAt(now.Add(-time.Minute)), now))
	require.Contains(t, formatResetHuman(ptrAt(now.Add(90*time.Minute)), now), "m")
}

func ptrAt(tm time.Time) *time.Time { return &tm }
