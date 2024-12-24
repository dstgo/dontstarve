package proc

import (
	"fmt"
	"time"
)

type Options struct {
	Name    string
	Args    []string
	WorkDir string
	Env     []string

	// create stdin pipe
	Stdin bool
	// create stdout pipe
	Stdout bool
	// create stderr pipe
	Stderr bool

	// max wait time before stop the process
	MaxWaitTime time.Duration
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

func WithStdin() Option {
	return func(opt *Options) {
		opt.Stdin = true
	}
}

func WithStdout() Option {
	return func(opt *Options) {
		opt.Stdout = true
	}
}

func WithStderr() Option {
	return func(opt *Options) {
		opt.Stderr = true
	}
}

func WithMaxWaitTime(t time.Duration) Option {
	return func(opt *Options) {
		opt.MaxWaitTime = t
	}
}
