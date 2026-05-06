package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var oauthPersistMu sync.Mutex

func mergeRefreshIntoCredentialsFile(path string, res RefreshOAuthResult) error {
	var root map[string]any
	if b, err := os.ReadFile(path); err == nil {
		if len(b) > 0 {
			if err := json.Unmarshal(b, &root); err != nil {
				return fmt.Errorf("decode %s: %w", path, err)
			}
		}
	}
	if root == nil {
		root = make(map[string]any)
	}
	co, _ := root["claudeAiOauth"].(map[string]any)
	if co == nil {
		co = make(map[string]any)
	}
	co["accessToken"] = res.AccessToken
	co["refreshToken"] = res.RefreshToken
	co["expiresAt"] = res.ExpiresAt.UnixMilli()
	root["claudeAiOauth"] = co

	data, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
