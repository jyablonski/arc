package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jyablonski/arc/internal/filemode"
)

const defaultCacheTTL = 45 * time.Second

type cacheFile struct {
	ExpiresAt time.Time       `json:"expires_at"`
	Report    AggregateReport `json:"report"`
}

func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "arc"), nil
}

func cachePath() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ai-usage.json"), nil
}

func ReadCache(now time.Time) (AggregateReport, bool, error) {
	path, err := cachePath()
	if err != nil {
		return AggregateReport{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AggregateReport{}, false, nil
		}
		return AggregateReport{}, false, err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return AggregateReport{}, false, nil
	}
	if now.After(cf.ExpiresAt) {
		return AggregateReport{}, false, nil
	}
	return cf.Report, true, nil
}

func WriteCache(now time.Time, report AggregateReport) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, filemode.Dir); err != nil {
		return fmt.Errorf("cache mkdir: %w", err)
	}
	cf := cacheFile{
		ExpiresAt: now.Add(defaultCacheTTL),
		Report:    report,
	}
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
