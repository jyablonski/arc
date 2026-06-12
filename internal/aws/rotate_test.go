package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rotateUserARN = "arn:aws:iam::123456789012:user/jacob"
	rotateOldKey  = "AKIAOLDKEY0000000000"
	rotateNewKey  = "AKIANEWKEY0000000000"
)

// awsCLIScript canned-responds to the IAM CLI calls RotateKeys makes via the
// run seam, so the whole flow can be driven without touching AWS.
type awsCLIScript struct {
	identityARN string
	keys        []string
	createErr   error
	deleteErr   error
}

func (s awsCLIScript) run(name string, args ...string) (string, error) {
	if name != "aws" || len(args) < 2 {
		return "", fmt.Errorf("unexpected command: %s %v", name, args)
	}
	switch {
	case args[0] == "sts" && args[1] == "get-caller-identity":
		return fmt.Sprintf(`{"Arn":%q}`, s.identityARN), nil
	case args[0] == "iam" && args[1] == "list-access-keys":
		meta := make([]map[string]string, 0, len(s.keys))
		for _, k := range s.keys {
			meta = append(meta, map[string]string{"AccessKeyId": k})
		}
		b, _ := json.Marshal(map[string]any{"AccessKeyMetadata": meta})
		return string(b), nil
	case args[0] == "iam" && args[1] == "create-access-key":
		if s.createErr != nil {
			return "", s.createErr
		}
		return fmt.Sprintf(`{"AccessKey":{"AccessKeyId":%q,"SecretAccessKey":"newsecret"}}`, rotateNewKey), nil
	case args[0] == "iam" && args[1] == "delete-access-key":
		return "", s.deleteErr
	}
	return "", fmt.Errorf("unexpected command: %s %v", name, args)
}

// realCopy makes the mocked restore (`cp backup credentials`) actually move the
// bytes, so tests can assert the on-disk file truly reverted to the old key.
func realCopy(name string, args ...string) error {
	if name == "cp" && len(args) == 2 {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		return os.WriteFile(args[1], data, 0o600)
	}
	return nil
}

func setSTS(t *testing.T, fn stsValidateFunc) {
	t.Helper()
	prev := stsCaller
	stsCaller = fn
	t.Cleanup(func() { stsCaller = prev })
}

func stsOK() stsValidateFunc {
	return func([]string) (string, string, error) {
		return fmt.Sprintf(`{"Arn":%q}`, rotateUserARN), "", nil
	}
}

func writeCreds(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "credentials")
	content := "[default]\naws_access_key_id = " + rotateOldKey + "\naws_secret_access_key = oldsecret\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func backupFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.bak"))
	require.NoError(t, err)
	return matches
}

// countSubcommand uses the moq's call tracking to count `aws iam <sub>` invocations.
func countSubcommand(mock *boundary.ShellRunnerMock, sub string) int {
	n := 0
	for _, c := range mock.RunCalls() {
		if c.Name == "aws" && len(c.Args) >= 2 && c.Args[1] == sub {
			n++
		}
	}
	return n
}

// When the freshly created key fails validation, the backup must be restored,
// the old key must NOT be deleted, and the error must surface — otherwise a
// rotation could silently strand the user with no working credentials.
func TestRotateKeys_validationFailsRestoresBackupAndKeepsOldKey(t *testing.T) {
	dir := t.TempDir()
	path := writeCreds(t, dir)

	mock := &boundary.ShellRunnerMock{
		RunFunc:            awsCLIScript{identityARN: rotateUserARN, keys: []string{rotateOldKey}}.run,
		RunInteractiveFunc: realCopy,
	}
	setRunner(t, mock)
	setSTS(t, func([]string) (string, string, error) {
		return "", "AccessDenied", errors.New("InvalidClientTokenId: The security token included in the request is invalid")
	})

	err := RotateKeys(path, "default", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed validation")

	// Restore was attempted: cp <backup> <credentials>.
	require.Len(t, mock.RunInteractiveCalls(), 1)
	assert.Equal(t, "cp", mock.RunInteractiveCalls()[0].Name)
	assert.Equal(t, path, mock.RunInteractiveCalls()[0].Args[1])

	// On-disk file reverted to the old key; the new key is gone.
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), rotateOldKey)
	assert.NotContains(t, string(data), rotateNewKey)

	// No destructive delete, and the backup is preserved for manual recovery.
	assert.Zero(t, countSubcommand(mock, "delete-access-key"), "old key must not be deleted on validation failure")
	assert.NotEmpty(t, backupFiles(t, dir), "backup must be kept when rotation fails")
}

// If validation fails AND the restore itself fails, the error must report both
// so the user knows the credentials file may be in a bad state.
func TestRotateKeys_restoreFailureSurfacesBothErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeCreds(t, dir)

	mock := &boundary.ShellRunnerMock{
		RunFunc: awsCLIScript{identityARN: rotateUserARN, keys: []string{rotateOldKey}}.run,
		RunInteractiveFunc: func(name string, args ...string) error {
			return errors.New("cp: permission denied")
		},
	}
	setRunner(t, mock)
	setSTS(t, func([]string) (string, string, error) {
		// AWS CLI reports the failure reason on stderr (2nd return), which is
		// what ValidateCredentials surfaces.
		return "", "InvalidClientTokenId: token invalid", errors.New("exit status 255")
	})

	err := RotateKeys(path, "default", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to restore backup")
	assert.Contains(t, err.Error(), "cp: permission denied")
	assert.Contains(t, err.Error(), "InvalidClientTokenId", "original validation error must be preserved")
	assert.Zero(t, countSubcommand(mock, "delete-access-key"))
}

// A failure to delete an old key AFTER a successful rotation is only a warning:
// the new key works, so the rotation has succeeded and must return nil.
func TestRotateKeys_deleteOldKeyFailureStillSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := writeCreds(t, dir)

	mock := &boundary.ShellRunnerMock{
		RunFunc: awsCLIScript{
			identityARN: rotateUserARN,
			keys:        []string{rotateOldKey},
			deleteErr:   errors.New("AccessDenied: not authorized to DeleteAccessKey"),
		}.run,
		RunInteractiveFunc: realCopy,
	}
	setRunner(t, mock)
	setSTS(t, stsOK())

	err := RotateKeys(path, "default", 1)
	require.NoError(t, err, "delete-old-key failure is non-fatal")

	// Rotation still applied: file holds the new key and delete was attempted.
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), rotateNewKey)
	assert.Equal(t, 1, countSubcommand(mock, "delete-access-key"))
}

// Happy path: new key validates, old key is deleted, backup is cleaned up.
func TestRotateKeys_success(t *testing.T) {
	dir := t.TempDir()
	path := writeCreds(t, dir)

	mock := &boundary.ShellRunnerMock{
		RunFunc:            awsCLIScript{identityARN: rotateUserARN, keys: []string{rotateOldKey}}.run,
		RunInteractiveFunc: realCopy,
	}
	setRunner(t, mock)
	setSTS(t, stsOK())

	err := RotateKeys(path, "default", 1)
	require.NoError(t, err)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), rotateNewKey)
	assert.NotContains(t, string(data), rotateOldKey)

	assert.Equal(t, 1, countSubcommand(mock, "delete-access-key"))
	assert.Empty(t, backupFiles(t, dir), "backup should be cleaned up after a successful rotation")

	// No restore on the happy path.
	assert.Empty(t, mock.RunInteractiveCalls())
}

// With no existing keys there is nothing to rotate: return early without
// creating or deleting anything.
func TestRotateKeys_noExistingKeysReturnsEarly(t *testing.T) {
	dir := t.TempDir()
	path := writeCreds(t, dir)

	mock := &boundary.ShellRunnerMock{
		RunFunc:            awsCLIScript{identityARN: rotateUserARN, keys: nil}.run,
		RunInteractiveFunc: realCopy,
	}
	setRunner(t, mock)

	err := RotateKeys(path, "default", 1)
	require.NoError(t, err)
	assert.Zero(t, countSubcommand(mock, "create-access-key"))
	assert.Zero(t, countSubcommand(mock, "delete-access-key"))
}

// A missing credentials file fails fast with a clear message.
func TestRotateKeys_missingCredentialsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist")

	err := RotateKeys(path, "default", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
