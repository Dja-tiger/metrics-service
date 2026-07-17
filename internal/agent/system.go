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

// SystemPollLoop collects gopsutil metrics periodically until the context is canceled.
func (a *Agent) SystemPollLoop(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	_ = a.PollSystemOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.PollSystemOnce()
		}
	}
}
