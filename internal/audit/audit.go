package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event describes a successful metrics update for audit sinks.
type Event struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// Observer receives audit events.
type Observer interface {
	Notify(ctx context.Context, event Event) error
}

// Notifier publishes audit events to all subscribed observers.
type Notifier struct {
	observers []Observer
}

// NewNotifier creates an audit notifier with optional initial observers.
func NewNotifier(observers ...Observer) *Notifier {
	return &Notifier{observers: observers}
}

// Subscribe adds an observer to the notifier.
func (n *Notifier) Subscribe(observer Observer) {
	if observer != nil {
		n.observers = append(n.observers, observer)
	}
}

// Notify sends an audit event to all observers and joins their errors.
func (n *Notifier) Notify(ctx context.Context, event Event) error {
	var result error
	for _, observer := range n.observers {
		if err := observer.Notify(ctx, event); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

// Enabled reports whether the notifier has at least one observer.
func (n *Notifier) Enabled() bool {
	return n != nil && len(n.observers) > 0
}

// FileObserver appends audit events to a local file.
type FileObserver struct {
	path string
	mu   sync.Mutex
}

// NewFileObserver creates a file audit observer.
func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

// Notify writes an audit event as a JSON line.
func (o *FileObserver) Notify(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(o.path)
	if dir != "." {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create audit directory: %w", err)
		}
	}

	file, err := os.OpenFile(o.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	defer file.Close()

	if _, err = file.Write(data); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// HTTPObserver sends audit events to a remote HTTP endpoint.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver creates an HTTP audit observer.
func NewHTTPObserver(url string, client *http.Client) *HTTPObserver {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &HTTPObserver{
		url:    url,
		client: client,
	}
}

// Notify posts an audit event as JSON to the configured URL.
func (o *HTTPObserver) Notify(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create audit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("send audit event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected audit response status: %d", resp.StatusCode)
	}
	return nil
}

// NewEvent creates an audit event for updated metric names and a request address.
func NewEvent(metricNames []string, remoteAddr string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Metrics:   metricNames,
		IPAddress: requestIP(remoteAddr),
	}
}

func requestIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}
