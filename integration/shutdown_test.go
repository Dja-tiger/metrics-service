package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

type process struct {
	cmd    *exec.Cmd
	done   chan error
	output bytes.Buffer
}

func start(t *testing.T, binary string, args ...string) *process {
	t.Helper()
	p := &process{cmd: exec.Command(binary, args...), done: make(chan error, 1)}
	p.cmd.Stdout = &p.output
	p.cmd.Stderr = &p.output
	// Ignore the developer's runtime configuration for isolated process tests.
	configVars := map[string]bool{"TRUSTED_SUBNET": true, "CONFIG": true, "ADDRESS": true, "REPORT_INTERVAL": true, "POLL_INTERVAL": true, "RATE_LIMIT": true, "KEY": true, "CRYPTO_KEY": true, "STORE_INTERVAL": true, "FILE_STORAGE_PATH": true, "STORE_FILE": true, "RESTORE": true, "DATABASE_DSN": true, "AUDIT_FILE": true, "AUDIT_URL": true}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !configVars[name] {
			p.cmd.Env = append(p.cmd.Env, entry)
		}
	}
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { p.done <- p.cmd.Wait(); close(p.done) }()
	t.Cleanup(func() { _ = p.cmd.Process.Kill(); <-p.done })
	return p
}

func (p *process) stop(t *testing.T, signal syscall.Signal) {
	t.Helper()
	if err := p.cmd.Process.Signal(signal); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("process exited with %v: %s", err, p.output.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("process did not shut down")
	}
}

func TestGracefulSignals(t *testing.T) {
	dir := t.TempDir()
	binaries := map[string]string{}
	for _, app := range []string{"server", "agent"} {
		binary := filepath.Join(dir, app)
		command := exec.Command("go", "build", "-o", binary, "./cmd/"+app)
		command.Dir = ".."
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		binaries[app] = binary
	}
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT} {
		t.Run(sig.String(), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			_ = listener.Close()
			file := filepath.Join(t.TempDir(), "metrics.json")
			server := start(t, binaries["server"], "-a="+addr, "-f="+file, "-i=3600", "-r=false")
			client := &http.Client{Timeout: time.Second}
			waitFor(t, func() bool {
				r, err := client.Get("http://" + addr + "/")
				if err != nil {
					return false
				}
				_, _ = io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
				return r.StatusCode == 200
			})
			agent := start(t, binaries["agent"], "-a="+addr, "-r=1", "-p=3600")
			waitFor(t, func() bool {
				r, err := client.Get("http://" + addr + "/value/counter/PollCount")
				if err != nil {
					return false
				}
				body, _ := io.ReadAll(r.Body)
				_ = r.Body.Close()
				return r.StatusCode == 200 && strings.TrimSpace(string(body)) == "1"
			})
			agent.stop(t, sig)
			response, err := client.Post("http://"+addr+"/update/counter/FinalCounter/17", "text/plain", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatalf("update status: %d", response.StatusCode)
			}
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Fatalf("periodic save unexpectedly ran: %v", err)
			}
			server.stop(t, sig)
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var metrics []models.Metrics
			if err := json.Unmarshal(data, &metrics); err != nil {
				t.Fatal(err)
			}
			counters := map[string]int64{}
			for _, metric := range metrics {
				if metric.Delta != nil {
					counters[metric.ID] = *metric.Delta
				}
			}
			if counters["FinalCounter"] != 17 || counters["PollCount"] != 1 {
				t.Fatalf("saved counters: %v", counters)
			}
		})
	}
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("process readiness timed out")
}
