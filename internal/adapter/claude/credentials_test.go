package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

func TestLoadCredentials(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"secret-token","expiresAt":1783940000000}}`)

	credentials, err := LoadCredentials(path, now)
	if err != nil {
		t.Fatal(err)
	}

	if credentials.AccessToken != "secret-token" {
		t.Fatalf("got token %q", credentials.AccessToken)
	}
}

func TestLoadCredentialsMissingIsNotConfigured(t *testing.T) {
	_, err := LoadCredentials(filepath.Join(t.TempDir(), "missing.json"), time.Now())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestLoadCredentialsRejectsMalformedAndMissingToken(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "malformed", data: "{"},
		{name: "missing token", data: `{"claudeAiOauth":{}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeCredentials(t, test.data)
			_, err := LoadCredentials(path, time.Now())
			if usage.ErrorKindOf(err) != usage.ErrorAuth {
				t.Fatalf("got %v, want auth error", err)
			}
		})
	}
}

func TestLoadCredentialsRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	path := writeCredentials(t, fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"secret-token","expiresAt":%d}}`, now.Add(-time.Minute).UnixMilli()))

	_, err := LoadCredentials(path, now)
	if usage.ErrorKindOf(err) != usage.ErrorAuth || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("got %v, want expired auth error", err)
	}
}

func TestLoadCredentialsRejectsBroadPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission checks are Unix-specific")
	}

	path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"secret-token"}}`)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadCredentials(path, time.Now())
	if err == nil || !strings.Contains(err.Error(), "accessible") {
		t.Fatalf("got %v, want permissions error", err)
	}
}

func writeCredentials(t *testing.T, data string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".credentials.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}
