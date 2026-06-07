package agent

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

func TestPollSystemOnceCollectsMetrics(t *testing.T) {
	originalVirtualMemory := readVirtualMemory
	originalCPUPercent := readCPUPercent
	defer func() {
		readVirtualMemory = originalVirtualMemory
		readCPUPercent = originalCPUPercent
	}()

	readVirtualMemory = func() (*mem.VirtualMemoryStat, error) {
		return &mem.VirtualMemoryStat{
			Total: 1024,
			Free:  256,
		}, nil
	}
	readCPUPercent = func(time.Duration, bool) ([]float64, error) {
		return []float64{10.5, 20.5}, nil
	}

	store := NewStore()
	a, err := NewAgent("http://localhost:8080", time.Second, time.Second, nil, store)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if err = a.PollSystemOnce(); err != nil {
		t.Fatalf("collect system metrics: %v", err)
	}

	gauges := store.SnapshotGauges()
	expected := map[string]float64{
		"TotalMemory":     1024,
		"FreeMemory":      256,
		"CPUutilization1": 10.5,
		"CPUutilization2": 20.5,
	}
	for name, want := range expected {
		if got, ok := gauges[name]; !ok || got != want {
			t.Fatalf("unexpected metric %s: got %v exists %t want %v", name, got, ok, want)
		}
	}
}
