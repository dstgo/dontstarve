package proc

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

type Options struct {
	Name    string
	Args    []string
	WorkDir string
	Env     []string
}

// Option apply option into *Options
type Option func(*Options)

func WithCommand(name string, args ...string) Option {
	return func(opt *Options) {
		opt.Name = name
		opt.Args = args
	}
}

func WithWorkDir(dir string) Option {
	return func(opt *Options) {
		opt.WorkDir = dir
	}
}

func WithEnv(env map[string]string) Option {
	return func(opts *Options) {
		var envs []string
		for k, v := range env {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
		opts.Env = envs
	}
}

// NewProc return a new Process
func NewProc(ctx context.Context, procOptions ...Option) (*Proc, error) {
	var opts Options
	for _, opt := range procOptions {
		opt(&opts)
	}

	procCmd := exec.CommandContext(ctx, opts.Name, opts.Args...)

	if len(opts.WorkDir) > 0 {
		procCmd.Dir = opts.WorkDir
	}
	if len(opts.Env) > 0 {
		procCmd.Env = opts.Env
	}

	// create pipe
	stdin, err := procCmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := procCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := procCmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	newProc := &Proc{
		cmd:        procCmd,
		StdinPipe:  stdin,
		StdoutPipe: stdout,
		StderrPipe: stderr,
		options:    opts,
	}

	return newProc, nil
}

// Proc represent a child process of dontstarve
type Proc struct {
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
	StdinPipe  io.WriteCloser
	StdoutPipe io.ReadCloser
	StderrPipe io.ReadCloser

	options Options
}

// Start starts the process but does not wait for it to complete.
func (p *Proc) Start() error {
	err := p.cmd.Start()
	if err != nil {
		return err
	}
	p.proc = p.cmd.Process

	p.createdAt = time.Now()

	pid := p.PID()
	if pid >= 0 {
		processInfo, err := process.NewProcess(int32(pid))
		if err != nil {
			return err
		}
		p.process = processInfo
	}
	return nil
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

// Kill causes the Process to exit immediately. Kill does not wait until
// the Process has actually exited
func (p *Proc) Kill() error {
	if p.proc == nil {
		return nil
	}
	return p.proc.Kill()
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
