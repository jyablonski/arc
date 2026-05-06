package ai_test

import (
	"context"
	"testing"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestProviderMock_smoke(t *testing.T) {
	m := &ai.ProviderMock{
		NameFunc: func() string { return "x" },
		UsageFunc: func(ctx context.Context) (ai.UsageReport, error) {
			return ai.UsageReport{}, nil
		},
	}
	require.Equal(t, "x", m.Name())
	_, err := m.Usage(context.Background())
	require.NoError(t, err)
}
