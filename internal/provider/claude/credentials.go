package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

var ErrNotConfigured = errors.New("claude credentials are not configured")

type Credentials struct {
	AccessToken string
	ExpiresAt   time.Time
}

type credentialsFile struct {
	ClaudeAIOAuth *oauthCredentials `json:"claudeAiOauth"`
}

type oauthCredentials struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func LoadCredentials(path string, now time.Time) (Credentials, error) {
	if path == "" {
		return Credentials{}, ErrNotConfigured
	}

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrNotConfigured
	}

	if err != nil {
		return Credentials{}, model.NewProviderError(model.ErrorAuth, fmt.Errorf("stat Claude credentials: %w", err))
	}

	if !info.Mode().IsRegular() {
		return Credentials{}, model.NewProviderError(model.ErrorAuth, errors.New("claude credentials path is not a regular file"))
	}

	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return Credentials{}, model.NewProviderError(model.ErrorAuth, errors.New("claude credentials file is accessible by group or other users"))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, model.NewProviderError(model.ErrorAuth, fmt.Errorf("read Claude credentials: %w", err))
	}

	var saved credentialsFile
	if err := json.Unmarshal(data, &saved); err != nil {
		return Credentials{}, model.NewProviderError(model.ErrorAuth, errors.New("claude credentials are malformed"))
	}

	if saved.ClaudeAIOAuth == nil || saved.ClaudeAIOAuth.AccessToken == "" {
		return Credentials{}, model.NewProviderError(model.ErrorAuth, errors.New("claude OAuth credentials are missing an access token"))
	}

	credentials := Credentials{AccessToken: saved.ClaudeAIOAuth.AccessToken}
	if saved.ClaudeAIOAuth.ExpiresAt > 0 {
		credentials.ExpiresAt = time.UnixMilli(saved.ClaudeAIOAuth.ExpiresAt)
		if !now.Before(credentials.ExpiresAt) {
			return Credentials{}, model.NewProviderError(model.ErrorAuth, errors.New("claude access token is expired; authenticate Claude Code again"))
		}
	}

	return credentials, nil
}
