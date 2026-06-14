package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/filemode"
)

const (
	pricingCacheFileName    = "ai-pricing.json"
	pricingOverrideFileName = "ai-pricing.json"
	// pricingOverrideSource labels costs that came from the hand-edited override
	// file when the entry itself does not set a source, so `pricing_source` makes
	// the layer visible in `arc ai tokens` output.
	pricingOverrideSource = "override"
)

// PricingCacheFile is the on-disk shape written by `arc ai tokens pricing`. It
// is read offline by every other tokens invocation.
type PricingCacheFile struct {
	FetchedAt time.Time             `json:"fetched_at"`
	Source    string                `json:"source"`
	Prices    map[string]ModelPrice `json:"prices"`
}

// PricingCachePath is ~/.cache/arc/ai-pricing.json (refreshed by the command).
func PricingCachePath() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pricingCacheFileName), nil
}

// PricingOverridePath is ~/.config/arc/ai-pricing.json (hand-edited by the user).
func PricingOverridePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "arc", pricingOverrideFileName), nil
}

// ReadPricingCacheFile returns the parsed cache file. ok is false when the file
// is absent; an error is only returned for unexpected read/parse failures so
// callers can surface a genuinely corrupt cache.
func ReadPricingCacheFile() (PricingCacheFile, bool, error) {
	path, err := PricingCachePath()
	if err != nil {
		return PricingCacheFile{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PricingCacheFile{}, false, nil
		}
		return PricingCacheFile{}, false, err
	}
	var cf PricingCacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return PricingCacheFile{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	return cf, true, nil
}

// ReadPricingCache returns just the cached price map, swallowing missing or
// corrupt files so the offline `arc ai tokens` path always degrades to the
// built-in defaults rather than failing.
func ReadPricingCache() (map[string]ModelPrice, bool) {
	cf, ok, err := ReadPricingCacheFile()
	if err != nil || !ok {
		return nil, false
	}
	return cf.Prices, len(cf.Prices) > 0
}

// WritePricingCache atomically writes the fetched prices to the cache path,
// mirroring the temp-then-rename pattern used by the usage cache.
func WritePricingCache(now time.Time, source string, prices map[string]ModelPrice) error {
	path, err := PricingCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), filemode.Dir); err != nil {
		return fmt.Errorf("pricing cache mkdir: %w", err)
	}
	cf := PricingCacheFile{FetchedAt: now, Source: source, Prices: prices}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadPricingOverride loads the optional hand-edited override map. Keys are
// normalized model IDs; entries without an explicit source are labelled
// "override" so the winning layer is visible in output. Missing or corrupt
// files yield ok=false rather than an error, keeping the read path resilient.
func ReadPricingOverride() (map[string]ModelPrice, bool) {
	path, err := PricingOverridePath()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var raw map[string]ModelPrice
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false
	}
	out := make(map[string]ModelPrice, len(raw))
	for model, price := range raw {
		key := normalizeModelID(model)
		if key == "" {
			continue
		}
		if strings.TrimSpace(price.Source) == "" {
			price.Source = pricingOverrideSource
		}
		out[key] = price
	}
	return out, len(out) > 0
}
