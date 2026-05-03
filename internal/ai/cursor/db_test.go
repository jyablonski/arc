package cursor

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

func TestReadAccessTokenFromDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.vscdb")
	dsn := "file:" + filepath.ToSlash(dbPath)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO ItemTable(key,value) VALUES('cursorAuth/accessToken','tok')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	tok, err := ReadAccessTokenFromDB(dbPath)
	require.NoError(t, err)
	require.Equal(t, "tok", tok)
}

func TestStateDBPath_linux(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip()
	}
	p := StateDBPath("/home/u")
	require.Contains(t, p, ".config/Cursor/User/globalStorage/state.vscdb")
}
