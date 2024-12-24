package proc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testProcess(t *testing.T, proc *Proc) {
	err := proc.Start()
	require.NoError(t, err)

	t.Logf("pid %d", proc.PID())
	t.Logf("name %s", proc.Name())
	t.Logf("cmd line %+v", proc.CMDLine())

	cwd, err := proc.Cwd()
	require.NoError(t, err)
	t.Logf("cwd %s", cwd)

	exe, err := proc.Exe()
	require.NoError(t, err)
	t.Logf("exe %s", exe)

	numFDs, err := proc.NumFDs()
	require.NoError(t, err)
	t.Logf("num fds %+v", numFDs)

	threads, err := proc.NumThreads()
	require.NoError(t, err)
	t.Logf("num threads %+v", threads)

	connections, err := proc.NumConnections()
	require.NoError(t, err)
	t.Logf("num connections %+v", connections)

	memoryInfo, err := proc.MemoryInfo()
	require.NoError(t, err)
	t.Logf("memory info: %+v", memoryInfo)

	cpuPercent, err := proc.CPUPercent()
	require.NoError(t, err)
	t.Logf("cpu percent: %+v", cpuPercent)

	ioCounters, err := proc.IOCounters()
	require.NoError(t, err)
	t.Logf("io counters: %+v", ioCounters)

	running, err := proc.IsRunning()
	require.NoError(t, err)
	require.True(t, running)

	createdAt, err := proc.CreatedAt()
	require.NoError(t, err)
	t.Logf("created at %s", createdAt)

	err = proc.Wait()
	t.Logf("wait error %v", err)

	isRunning, err := proc.IsRunning()
	require.NoError(t, err)
	require.False(t, isRunning)

	t.Logf("exit at %d", proc.ExitCode())
}

func TestNewProcOnce(t *testing.T) {
	sample := []struct {
		ctx     context.Context
		options []Option
	}{
		{
			ctx: context.Background(),
			options: []Option{
				WithCommand("curl", "https://baidu.com"),
				WithWorkDir("/root/"),
			},
		},
		{
			ctx: context.Background(),
			options: []Option{
				WithCommand("go", "version"),
				WithWorkDir("/root/"),
			},
		},
		{
			ctx: context.Background(),
			options: []Option{
				WithCommand("wget", "https://cdn.jsdelivr.net/npm/vue@3.5.13/dist/vue.global.min.js"),
				WithWorkDir("/root/"),
			},
		},
	}

	for _, s := range sample {
		proc, err := NewProc(s.ctx, s.options...)
		require.NoError(t, err)
		testProcess(t, proc)
	}
}

func TestKillProcess(t *testing.T) {

	sh := `#!/bin/bash

# Infinite loop
while true; do
    # Print the current time
    echo "Current Time: $(date)"

    # Print system information
    echo "System Information:"
    # CPU usage
    echo "CPU Usage: $(top -bn1 | grep "Cpu(s)" | awk '{print $2 + $4"%"}')"
    # Memory usage
    mem_info=$(free -m | awk 'NR==2{printf "Memory Usage: %.2f%% (Used: %sMB / Total: %sMB)", $3*100/$2, $3, $2}')
    echo "$mem_info"

    # Add an empty line for separation
    echo ""

    # Wait for 1 second
    sleep 1
done
`
	ctx := context.Background()
	proc, err := NewProc(ctx,
		WithCommand("bash", "-c", sh),
	)
	require.NoError(t, err)
	t.Log(proc.Start())

	go func() {
		time.Sleep(5 * time.Second)
		t.Log(proc.Kill())
	}()

	t.Log(proc.Wait())
}

func TestTerminateProcess(t *testing.T) {

	sh := `#!/bin/bash

# Infinite loop
while true; do
    # Print the current time
    echo "Current Time: $(date)"

    # Print system information
    echo "System Information:"
    # CPU usage
    echo "CPU Usage: $(top -bn1 | grep "Cpu(s)" | awk '{print $2 + $4"%"}')"
    # Memory usage
    mem_info=$(free -m | awk 'NR==2{printf "Memory Usage: %.2f%% (Used: %sMB / Total: %sMB)", $3*100/$2, $3, $2}')
    echo "$mem_info"

    # Add an empty line for separation
    echo ""

    # Wait for 1 second
    sleep 1
done
`
	ctx := context.Background()
	proc, err := NewProc(ctx,
		WithCommand("bash", "-c", sh),
	)
	require.NoError(t, err)
	t.Log(proc.Start())

	go func() {
		time.Sleep(5 * time.Second)
		t.Log(proc.Terminate())
	}()

	t.Log(proc.Wait())
}
