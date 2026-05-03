package cursor

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"
)

func StateDBPath(homeDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb")
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "Cursor", "User", "globalStorage", "state.vscdb")
	default:
		return filepath.Join(homeDir, ".config", "Cursor", "User", "globalStorage", "state.vscdb")
	}
}

func ReadAccessTokenFromDB(dbPath string) (string, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return "", err
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(abs))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return "", fmt.Errorf("sqlite open: %w", err)
	}
	defer db.Close()

	var val string
	err = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken' LIMIT 1`).Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no cursorAuth/accessToken in %s — is Cursor logged in?", dbPath)
		}
		return "", fmt.Errorf("read token from sqlite: %w", err)
	}
	return val, nil
}
