package proc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/valyala/bytebufferpool"
	"golang.org/x/sync/errgroup"
)

// NewProc return a new Process
func NewProc(ctx context.Context, procOptions ...Option) (*Proc, error) {
	var opts Options
	for _, opt := range procOptions {
		opt(&opts)
	}

	procCmd := exec.CommandContext(ctx, opts.Name, opts.Args...)
	newProc := &Proc{cmd: procCmd}

	if len(opts.WorkDir) > 0 {
		procCmd.Dir = opts.WorkDir
	}
	if len(opts.Env) > 0 {
		procCmd.Env = opts.Env
	}

	if opts.Stdin {
		stdin, err := procCmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		newProc.stdinPipe = stdin
		newProc.stdinChs = make(map[string]*Stream)
	}

	if opts.Stdout {
		stdout, err := procCmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		newProc.stdoutPipe = stdout
		newProc.stdoutChs = make(map[string]*Stream)
	}

	if opts.Stderr {
		stderr, err := procCmd.StderrPipe()
		if err != nil {
			return nil, err
		}
		newProc.stderrPipe = stderr
		newProc.stderrChs = make(map[string]*Stream)
	}

	if opts.Stdin || opts.Stdout || opts.Stderr {
		group, groupCtx := errgroup.WithContext(ctx)
		newProc.group = group
		newProc.ctx = groupCtx

		workerPool, err := ants.NewPool(20, ants.WithNonblocking(true))
		if err != nil {
			return nil, err
		}
		newProc.workerPool = workerPool
	}

	if newProc.ctx == nil {
		newProc.ctx = ctx
	}

	ctx, cancelFunc := context.WithCancel(newProc.ctx)
	newProc.ctx = ctx
	newProc.cancel = cancelFunc

	return newProc, nil
}

// Proc represent a child process of dontstarve
type Proc struct {
	ctx    context.Context
	cancel context.CancelFunc

	// start command
	cmd *exec.Cmd
	// running process instance
	proc *os.Process
	// running process info, it will be set after Start
	process *process.Process
	// exited state info by Wait
	state *os.ProcessState

	createdAt time.Time

	// process pipe
	stdinMu   sync.Mutex
	stdinPipe io.WriteCloser
	stdinChs  map[string]*Stream

	stdoutPipe io.ReadCloser
	stdoutChs  map[string]*Stream

	stderrPipe io.ReadCloser
	stderrChs  map[string]*Stream

	// group and pool
	group      *errgroup.Group
	workerPool *ants.Pool
	bufferPool bytebufferpool.Pool

	options Options
}

func (p *Proc) start() error {
	pid := p.PID()
	if pid >= 0 {
		processInfo, err := process.NewProcess(int32(pid))
		if err != nil {
			return err
		}
		p.process = processInfo
	}

	p.listenStdinPipe(p.ctx)
	p.listenStdoutPipe(p.ctx)
	p.listenStderrPipe(p.ctx)

	return nil
}

// Start starts the process but does not wait for it to complete.
func (p *Proc) Start() error {
	// start the process
	err := p.cmd.Start()
	if err != nil {
		return err
	}
	p.proc = p.cmd.Process
	p.createdAt = time.Now()

	return p.start()
}

// Wait waits for the process to exit and waits for any copying to
// stdin or copying from stdout or stderr to complete.
func (p *Proc) Wait() error {
	err := p.cmd.Wait()
	p.state = p.cmd.ProcessState
	if err != nil {
		return err
	}
	return nil
}

func (p *Proc) close() error {
	for _, stream := range p.stdinChs {
		stream.Close()
	}
	p.stdinPipe.Close()
	for _, stream := range p.stdoutChs {
		stream.Close()
	}
	p.stdoutPipe.Close()
	for _, stream := range p.stderrChs {
		stream.Close()
	}
	p.stderrPipe.Close()

	defer p.workerPool.Release()

	p.cancel()

	if p.options.MaxWaitTime == 0 {
		return p.group.Wait()
	}

	done := make(chan error)
	go func() {
		done <- p.cmd.Wait()
		close(done)
	}()

	select {
	case <-time.After(p.options.MaxWaitTime):
		return context.DeadlineExceeded
	case err := <-done:
		return err
	}
}

// Terminate closed the process with syscall.SIGTERM, should not call concurrently
func (p *Proc) Terminate() error {
	if p.proc == nil {
		return nil
	}

	closeErr := p.close()

	signalErr := p.Signal(syscall.SIGTERM)

	return errors.Join(closeErr, signalErr)
}

// Interrupt closed the process with syscall.SIGINT, should not call concurrently
func (p *Proc) Interrupt() error {
	if p.proc == nil {
		return nil
	}

	closeErr := p.close()

	signalErr := p.Signal(syscall.SIGINT)

	return errors.Join(closeErr, signalErr)
}

// Kill causes the Process to exit immediately. Kill does not wait until
// the Process has actually exited
func (p *Proc) Kill() error {
	if p.proc == nil {
		return nil
	}

	closeErr := p.close()

	signalErr := p.Signal(syscall.SIGKILL)

	return errors.Join(closeErr, signalErr)
}

// Signal sends a signal to the Process.
func (p *Proc) Signal(signal syscall.Signal) error {
	if p.proc == nil {
		return nil
	}
	return p.proc.Signal(signal)
}

// ExitCode returns the exit code of the exited process, or -1
// if the process hasn't exited or was terminated by a signal.
func (p *Proc) ExitCode() int {
	if p.state == nil {
		return -1
	}
	return p.state.ExitCode()
}

// PID returns the process id of the process.
func (p *Proc) PID() int {
	if p.proc == nil {
		return -1
	}
	return p.proc.Pid
}

// Name returns name of the process.
func (p *Proc) Name() string {
	return p.options.Name
}

// CMDLine return cmd line args for the process
func (p *Proc) CMDLine() []string {
	return append([]string{p.Name()}, p.options.Args...)
}

// Cwd returns current working directory of the process.
func (p *Proc) Cwd() (string, error) {
	if p.process == nil {
		return "", nil
	}
	return p.process.Cwd()
}

// Exe returns executable path of the process.
func (p *Proc) Exe() (string, error) {
	if p.process == nil {
		return "", nil
	}
	return p.process.Exe()
}

// CreatedAt return the time at process creating
func (p *Proc) CreatedAt() (time.Time, error) {
	return p.createdAt, nil
}

// IsRunning returns whether the process is still running or not.
func (p *Proc) IsRunning() (bool, error) {
	if p.process == nil {
		return false, nil
	}

	isRunning, err := p.process.IsRunning()
	if err != nil {
		return false, err
	}
	return isRunning, nil
}

// MemoryInfo returns generic process memory information, such as RSS and VMS.
func (p *Proc) MemoryInfo() (*process.MemoryInfoStat, error) {
	if p.process == nil {
		return &process.MemoryInfoStat{}, nil
	}
	return p.process.MemoryInfo()
}

// CPUPercent returns how many percent of the CPU time this process uses
func (p *Proc) CPUPercent() (float64, error) {
	if p.process == nil {
		return 0, nil
	}
	return p.process.CPUPercent()
}

// IOCounters returns IO Counters.
func (p *Proc) IOCounters() (*process.IOCountersStat, error) {
	if p.process == nil {
		return &process.IOCountersStat{}, nil
	}
	return p.process.IOCounters()
}

// NumConnections  the number of Connections used by the process.
// This returns all kind of the connection. This means TCP, UDP or UNIX.
func (p *Proc) NumConnections() (int, error) {
	if p.process == nil {
		return 0, nil
	}
	connections, err := p.process.Connections()
	if err != nil {
		return 0, err
	}
	return len(connections), nil
}

// NumFDs returns the number of File Descriptors used by the process.
func (p *Proc) NumFDs() (int32, error) {
	if p.process == nil {
		return 0, nil
	}
	return p.process.NumFDs()
}

// NumThreads returns the number of threads used by the process.
func (p *Proc) NumThreads() (int32, error) {
	if p.process == nil {
		return 0, nil
	}
	return p.process.NumThreads()
}
