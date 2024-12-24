package proc

import (
	"bufio"
	"context"
	"io"
	"os"
	"syscall"
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

	io.Copy(os.Stdout, proc.stdoutPipe)
	io.Copy(os.Stderr, proc.stderrPipe)

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

func TestNewProcDaemon(t *testing.T) {
	ctx := context.Background()
	proc, err := NewProc(
		ctx,
		WithCommand("/root/httpserver"),
		WithWorkDir("/root/"),
	)
	require.NoError(t, err)

	err = proc.Start()
	require.NoError(t, err)

	t.Logf("pid %d", proc.PID())
	t.Logf("name %s", proc.Name())
	t.Logf("cmd line %+v", proc.CMDLine())

	go func() {
		scanner := bufio.NewScanner(proc.stderrPipe)
		for scanner.Scan() {
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
			t.Log(scanner.Text())
		}
	}()

	go func() {
		for range time.After(time.Second * 60) {
			err := proc.Signal(syscall.SIGTERM)
			t.Logf("kill err %v", err)
		}
	}()

	err = proc.Wait()
	t.Logf("wait error %v", err)
}
