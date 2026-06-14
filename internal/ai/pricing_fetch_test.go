package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const litellmFixture = `{
  "claude-sonnet-4-5": {
    "input_cost_per_token": 3e-06,
    "output_cost_per_token": 1.5e-05,
    "cache_read_input_token_cost": 3e-07,
    "cache_creation_input_token_cost": 3.75e-06,
    "litellm_provider": "anthropic",
    "mode": "chat"
  },
  "gpt-5": {
    "input_cost_per_token": 1.25e-06,
    "output_cost_per_token": 1e-05,
    "cache_read_input_token_cost": 1.25e-07,
    "litellm_provider": "openai",
    "mode": "chat"
  },
  "gemini-2.5-pro": {
    "input_cost_per_token": 1.25e-06,
    "output_cost_per_token": 1e-05,
    "litellm_provider": "vertex_ai-language-models"
  },
  "embed-only": {
    "litellm_provider": "openai"
  }
}`

func TestFetchPricing_convertsAndFilters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(litellmFixture))
	}))
	defer ts.Close()

	prices, err := FetchPricing(context.Background(), ts.URL, ts.Client())
	require.NoError(t, err)

	// Only anthropic/openai entries with a usable cost survive; the gemini entry
	// and the cost-less embed entry are dropped.
	require.Len(t, prices, 2)

	sonnet := prices["claude-sonnet-4-5"]
	require.InDelta(t, 3.0, sonnet.InputPerMillion, 1e-9)
	require.InDelta(t, 15.0, sonnet.OutputPerMillion, 1e-9)
	require.InDelta(t, 0.30, sonnet.CacheReadPerMillion, 1e-9)
	require.InDelta(t, 3.75, sonnet.CacheWritePerMillion, 1e-9)
	require.Equal(t, "litellm:anthropic", sonnet.Source)

	gpt := prices["gpt-5"]
	require.InDelta(t, 1.25, gpt.InputPerMillion, 1e-9)
	require.Equal(t, "litellm:openai", gpt.Source)
}

func TestFetchPricing_httpError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := FetchPricing(context.Background(), ts.URL, ts.Client())
	require.Error(t, err)
}

func TestFetchPricing_emptyResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"gemini-2.5-pro":{"input_cost_per_token":1e-06,"litellm_provider":"vertex_ai"}}`))
	}))
	defer ts.Close()

	_, err := FetchPricing(context.Background(), ts.URL, ts.Client())
	require.Error(t, err)
}
