package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Notification is one push message.
type Notification struct {
	Title    string
	Message  string
	Priority int
	Tags     []string
	Click    string
	Attach   string
}

type Notifier interface {
	Send(ctx context.Context, n Notification) error
}

// Ntfy publishes via ntfy's JSON API. JSON rather than headers, because
// titles carry € and umlauts, which HTTP headers can't.
type Ntfy struct {
	Server string
	Topic  string
	Client *http.Client
}

// NewNtfy splits a topic URL like https://ntfy.sh/my-topic into server and topic.
func NewNtfy(topicURL string) (*Ntfy, error) {
	u, err := url.Parse(strings.TrimSpace(topicURL))
	if err != nil {
		return nil, err
	}
	topic := strings.Trim(u.Path, "/")
	if u.Scheme == "" || u.Host == "" || topic == "" || strings.Contains(topic, "/") {
		return nil, fmt.Errorf("NTFY_URL %q: erwartet https://server/topic", topicURL)
	}
	return &Ntfy{
		Server: u.Scheme + "://" + u.Host,
		Topic:  topic,
		Client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (n *Ntfy) Send(ctx context.Context, msg Notification) error {
	body, err := json.Marshal(map[string]any{
		"topic":    n.Topic,
		"title":    msg.Title,
		"message":  msg.Message,
		"priority": msg.Priority,
		"tags":     msg.Tags,
		"click":    msg.Click,
		"attach":   msg.Attach,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.Server+"/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ntfy: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
