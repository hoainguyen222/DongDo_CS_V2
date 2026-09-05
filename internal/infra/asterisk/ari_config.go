// ari_config.go — ARI connection configuration derived from app config.
package asterisk

import (
	"fmt"
	"net/url"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
)

// ARIConfig holds the settings needed to connect to Asterisk ARI.
type ARIConfig struct {
	BaseURL      string
	WSURL        string
	Username     string
	Password     string
	AppName      string
	ReconnectSec int
}

// FromConfig converts the app-level AsteriskARIConfig into the infra-local
// ARIConfig, deriving the WebSocket URL from BaseURL if WSURL is not set.
func ARIConfigFromConfig(cfg config.AsteriskARIConfig) (ARIConfig, error) {
	if cfg.BaseURL == "" {
		return ARIConfig{}, fmt.Errorf("asterisk ARI: base URL is required")
	}
	if cfg.Username == "" {
		return ARIConfig{}, fmt.Errorf("asterisk ARI: username is required")
	}
	if cfg.Password == "" {
		return ARIConfig{}, fmt.Errorf("asterisk ARI: password is required")
	}
	if cfg.AppName == "" {
		cfg.AppName = "dongdo-ivr"
	}
	if cfg.ReconnectSec <= 0 {
		cfg.ReconnectSec = 5
	}

	wsURL := cfg.WSURL
	if wsURL == "" {
		// Derive ws:// from http://baseURL
		u, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return ARIConfig{}, fmt.Errorf("asterisk ARI: invalid base URL %q: %w", cfg.BaseURL, err)
		}
		u.Scheme = "ws"
		if u.Scheme == "https" {
			u.Scheme = "wss"
		}
		u.Path = "/ari/events"
		q := u.Query()
		q.Set("app", cfg.AppName)
		q.Set("api_key", cfg.Username+":"+cfg.Password)
		q.Set("debug", "false")
		u.RawQuery = q.Encode()
		wsURL = u.String()
	}

	return ARIConfig{
		BaseURL:      cfg.BaseURL,
		WSURL:        wsURL,
		Username:     cfg.Username,
		Password:     cfg.Password,
		AppName:      cfg.AppName,
		ReconnectSec: cfg.ReconnectSec,
	}, nil
}
