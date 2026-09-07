package asterisk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type ARIClient struct {
	baseURL  string
	username string
	password string
	appName  string
	client   *http.Client
}

func NewARIClient(baseURL, username, password, appName string) *ARIClient {
	return &ARIClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		appName:  appName,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// OriginateCall creates a channel to endpoint via ARI REST
func (c *ARIClient) OriginateChannel(ctx context.Context, endpoint, channelID, callerID string) (string, error) {
	url := fmt.Sprintf("%s/ari/channels/%s?endpoint=%s&app=%s&callerId=%s",
		c.baseURL, channelID, endpoint, c.appName, callerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("endpoint", endpoint).Msg("Failed ARI Originate request")
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ARI Originate error status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().Str("channel_id", channelID).Str("endpoint", endpoint).Msg("ARI Originate channel success")
	return channelID, nil
}

// CreateBridge creates a mixing bridge in Asterisk
func (c *ARIClient) CreateBridge(ctx context.Context, bridgeID string) (string, error) {
	url := fmt.Sprintf("%s/ari/bridges/%s?type=mixing&name=%s", c.baseURL, bridgeID, bridgeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ARI CreateBridge status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().Str("bridge_id", bridgeID).Msg("ARI Bridge created successfully")
	return bridgeID, nil
}

// AddChannelToBridge adds a channel to a bridge
func (c *ARIClient) AddChannelToBridge(ctx context.Context, bridgeID, channelID string) error {
	url := fmt.Sprintf("%s/ari/bridges/%s/addChannel?channel=%s", c.baseURL, bridgeID, channelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ARI AddChannelToBridge status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().Str("bridge_id", bridgeID).Str("channel_id", channelID).Msg("Added channel to Asterisk bridge")
	return nil
}

// HangupChannel hangs up an active channel
func (c *ARIClient) HangupChannel(ctx context.Context, channelID string) error {
	url := fmt.Sprintf("%s/ari/channels/%s", c.baseURL, channelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ARI Hangup error status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().Str("channel_id", channelID).Msg("ARI Hangup channel requested")
	return nil
}

// StartRecording starts recording on a bridge
func (c *ARIClient) StartRecording(ctx context.Context, bridgeID, name string) error {
	url := fmt.Sprintf("%s/ari/bridges/%s/record?name=%s&format=wav", c.baseURL, bridgeID, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Info().Str("bridge_id", bridgeID).Str("name", name).Msg("ARI StartRecording bridge")
	return nil
}
