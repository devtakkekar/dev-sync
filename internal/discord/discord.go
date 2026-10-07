package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devsync/dev-sync/internal/config"
	"github.com/devsync/dev-sync/internal/logging"
)

// EmbedField represents a field inside a Discord webhook embed.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Embed represents a Discord webhook embed structure.
type Embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
}

type EmbedFooter struct {
	Text string `json:"text"`
}

// WebhookPayload represents the message payload sent to Discord.
type WebhookPayload struct {
	Username  string  `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Content   string  `json:"content,omitempty"`
	Embeds    []Embed `json:"embeds"`
}

const (
	ColorSuccess = 0x2ecc71 // Green
	ColorError   = 0xe74c3c // Red
	ColorWarning = 0xf39c12 // Yellow/Orange
	ColorInfo    = 0x3498db // Blue
)

// Client handles sending Discord webhook notifications with rate limiting and aggregation.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	mu         sync.Mutex
	lastSent   time.Time
}

// NewClient creates a new Discord webhook client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendEmbed transmits an embed to Discord respecting rate limits.
func (c *Client) SendEmbed(ctx context.Context, embed Embed) error {
	webhookURL := c.cfg.EffectiveWebhookURL()
	if !c.cfg.Discord.Enabled || webhookURL == "" {
		return nil
	}

	c.mu.Lock()
	// Rate limit: ensure at least 500ms between calls
	now := time.Now()
	elapsed := now.Sub(c.lastSent)
	if elapsed < 500*time.Millisecond {
		time.Sleep(500*time.Millisecond - elapsed)
	}
	c.lastSent = time.Now()
	c.mu.Unlock()

	embed.Timestamp = time.Now().UTC().Format(time.RFC3339)
	embed.Footer = &EmbedFooter{Text: "Dev-Sync • Filesystem Replicator"}

	payload := WebhookPayload{
		Username: "Dev-Sync",
		Embeds:   []Embed{embed},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	// Retry loop for rate limits
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to contact discord webhook: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logging.Debug("Discord notification dispatched successfully")
			return nil
		}

		// Handle Discord rate limit (429)
		if resp.StatusCode == 429 {
			retryAfterStr := resp.Header.Get("Retry-After")
			retrySeconds, _ := strconv.ParseFloat(retryAfterStr, 64)
			if retrySeconds <= 0 {
				retrySeconds = 1.0
			}
			logging.Warn("Discord rate limited, waiting %.1f seconds...", retrySeconds)
			select {
			case <-time.After(time.Duration(retrySeconds*1000) * time.Millisecond):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return fmt.Errorf("discord webhook returned HTTP %d", resp.StatusCode)
	}

	return fmt.Errorf("discord webhook exceeded retry attempts")
}

// NotifySyncCompleted sends a completed sync notification.
func (c *Client) NotifySyncCompleted(ctx context.Context, projectName string, replicas []string, created, modified, deleted int, duration time.Duration) {
	if !c.cfg.Discord.Events.SyncCompleted {
		return
	}

	repList := strings.Join(replicas, ", ")
	if repList == "" {
		repList = "(none)"
	}

	embed := Embed{
		Title:       "✓ Synchronization Completed",
		Description: fmt.Sprintf("Project **%s** synchronized to %d replica(s)", projectName, len(replicas)),
		Color:       ColorSuccess,
		Fields: []EmbedField{
			{Name: "Replicas", Value: repList, Inline: false},
			{Name: "Created", Value: strconv.Itoa(created), Inline: true},
			{Name: "Modified", Value: strconv.Itoa(modified), Inline: true},
			{Name: "Deleted", Value: strconv.Itoa(deleted), Inline: true},
			{Name: "Duration", Value: fmt.Sprintf("%.2fs", duration.Seconds()), Inline: true},
		},
	}

	_ = c.SendEmbed(ctx, embed)
}

// NotifySyncFailed sends a sync failure notification.
func (c *Client) NotifySyncFailed(ctx context.Context, projectName, replicaID, reason string) {
	if !c.cfg.Discord.Events.SyncFailed {
		return
	}

	embed := Embed{
		Title:       "✗ Synchronization Failed",
		Description: fmt.Sprintf("Project **%s** encountered an error replicating to **%s**", projectName, replicaID),
		Color:       ColorError,
		Fields: []EmbedField{
			{Name: "Replica", Value: replicaID, Inline: true},
			{Name: "Reason", Value: reason, Inline: false},
		},
	}

	_ = c.SendEmbed(ctx, embed)
}

// NotifyDriftDetected sends a drift detected notification.
func (c *Client) NotifyDriftDetected(ctx context.Context, projectName, replicaID string, modified, missing, extra []string) {
	if !c.cfg.Discord.Events.DriftDetected {
		return
	}

	fields := []EmbedField{
		{Name: "Replica", Value: replicaID, Inline: true},
	}
	if len(modified) > 0 {
		fields = append(fields, EmbedField{Name: "Modified", Value: strings.Join(modified, "\n"), Inline: false})
	}
	if len(missing) > 0 {
		fields = append(fields, EmbedField{Name: "Missing", Value: strings.Join(missing, "\n"), Inline: false})
	}
	if len(extra) > 0 {
		fields = append(fields, EmbedField{Name: "Extra", Value: strings.Join(extra, "\n"), Inline: false})
	}

	embed := Embed{
		Title:       "⚠ Replica Drift Detected",
		Description: fmt.Sprintf("Drift was detected on replica **%s** in project **%s**", replicaID, projectName),
		Color:       ColorWarning,
		Fields:      fields,
	}

	_ = c.SendEmbed(ctx, embed)
}
