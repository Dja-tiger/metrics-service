package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileObserverAppendsEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	observer := NewFileObserver(path)

	event := Event{
		Timestamp: 123,
		Metrics:   []string{"Alloc", "Frees"},
		IPAddress: "192.168.0.42",
	}
	if err := observer.Notify(context.Background(), event); err != nil {
		t.Fatalf("write first event: %v", err)
	}
	if err := observer.Notify(context.Background(), event); err != nil {
		t.Fatalf("write second event: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
		var got Event
		if err = json.Unmarshal(scanner.Bytes(), &got); err != nil {
			t.Fatalf("decode audit line: %v", err)
		}
		if !reflect.DeepEqual(got, event) {
			t.Fatalf("unexpected event: got %#v want %#v", got, event)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatalf("scan audit file: %v", err)
	}
	if lines != 2 {
		t.Fatalf("unexpected line count: got %d want 2", lines)
	}
}

func TestHTTPObserverPostsEvent(t *testing.T) {
	event := Event{
		Timestamp: 123,
		Metrics:   []string{"Alloc"},
		IPAddress: "192.168.0.42",
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}

		var got Event
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode audit request: %v", err)
		}
		if !reflect.DeepEqual(got, event) {
			t.Fatalf("unexpected event: got %#v want %#v", got, event)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	observer := NewHTTPObserver(server.URL, server.Client())
	if err := observer.Notify(context.Background(), event); err != nil {
		t.Fatalf("post audit event: %v", err)
	}
	if requests != 1 {
		t.Fatalf("unexpected request count: got %d want 1", requests)
	}
}

func TestNotifierSendsToObservers(t *testing.T) {
	first := &recordingObserver{}
	second := &recordingObserver{}
	notifier := NewNotifier(first, second)

	event := Event{Metrics: []string{"Alloc"}}
	if err := notifier.Notify(context.Background(), event); err != nil {
		t.Fatalf("notify observers: %v", err)
	}

	if len(first.events) != 1 || len(second.events) != 1 {
		t.Fatalf("expected both observers to receive event")
	}
}

func TestNewEventExtractsIP(t *testing.T) {
	event := NewEvent([]string{"Alloc"}, "192.168.0.42:12345")
	if event.IPAddress != "192.168.0.42" {
		t.Fatalf("unexpected ip address: %s", event.IPAddress)
	}
	if !reflect.DeepEqual(event.Metrics, []string{"Alloc"}) {
		t.Fatalf("unexpected metrics: %#v", event.Metrics)
	}
	if event.Timestamp == 0 {
		t.Fatal("expected timestamp to be set")
	}
}

type recordingObserver struct {
	events []Event
}

func (o *recordingObserver) Notify(ctx context.Context, event Event) error {
	o.events = append(o.events, event)
	return nil
}
