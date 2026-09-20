package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

var (
	readVirtualMemory = mem.VirtualMemory
	readCPUPercent    = cpu.Percent
)

// PollSystemOnce collects gopsutil memory and CPU metrics once.
func (a *Agent) PollSystemOnce() error {
	memoryStats, memoryErr := readVirtualMemory()
	if memoryErr == nil {
		a.store.SetGauge("TotalMemory", float64(memoryStats.Total))
		a.store.SetGauge("FreeMemory", float64(memoryStats.Free))
	}

	cpuUtilization, cpuErr := readCPUPercent(0, true)
	if cpuErr == nil {
		for index, value := range cpuUtilization {
			a.store.SetGauge(fmt.Sprintf("CPUutilization%d", index+1), value)
		}
	}

	return errors.Join(memoryErr, cpuErr)
}

// SystemPollLoop collects gopsutil metrics until cancellation or a collection error.
func (a *Agent) SystemPollLoop(ctx context.Context) error {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	if err := a.PollSystemOnce(); err != nil {
		return fmt.Errorf("collect system metrics: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := a.PollSystemOnce(); err != nil {
				return fmt.Errorf("collect system metrics: %w", err)
			}
		}
	}
}
