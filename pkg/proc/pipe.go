package proc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"
)

type Stream = Channel[[]byte]

// StdinPipe return a named stream pipe with stdin
func (p *Proc) StdinPipe(name string) *Stream {
	if p.PID() != -1 {
		panic(fmt.Sprintf("bind pipe after process started: %s", name))
	}

	if !p.options.Stdin {
		return nil
	}

	ch := MakeChannel[[]byte](0)
	p.stdinChs[name] = ch

	return ch
}

// StdoutPipe return a named stream pipe with stdout
func (p *Proc) StdoutPipe(name string) *Stream {
	if p.PID() != -1 {
		panic(fmt.Sprintf("bind pipe after process started: %s", name))
	}

	if !p.options.Stdout {
		return nil
	}

	ch := MakeChannel[[]byte](0)
	p.stdoutChs[name] = ch

	return ch
}

// StderrPipe return a named stream pipe with stderr
func (p *Proc) StderrPipe(name string) *Stream {
	if p.PID() != -1 {
		panic(fmt.Sprintf("bind pipe after process started: %s", name))
	}

	if !p.options.Stderr {
		return nil
	}

	ch := MakeChannel[[]byte](0)
	p.stderrChs[name] = ch

	return ch
}

func (p *Proc) listenStdinPipe(ctx context.Context) {
	if !p.options.Stdin {
		return
	}

	// create goroutine to receive stdin stream
	for name, stdinCh := range p.stdinChs {
		p.group.Go(func() error {
			for {
				if done, err := isCtxDone(ctx); done {
					return err
				}

				bs, ok := stdinCh.Recv()
				if !ok {
					return nil
				}

				p.stdinMu.Lock()
				_, err := p.stdinPipe.Write(bs)
				p.stdinMu.Unlock()

				if err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			}
		})
	}
}

func (p *Proc) listenStdoutPipe(ctx context.Context) {
	if !p.options.Stdout {
		return
	}

	p.listenOutStream(ctx, p.stdoutPipe, p.stdoutChs)
}

func (p *Proc) listenStderrPipe(ctx context.Context) {
	if !p.options.Stderr {
		return
	}

	p.listenOutStream(ctx, p.stderrPipe, p.stderrChs)
}

func (p *Proc) listenOutStream(ctx context.Context, readCloser io.ReadCloser, readChs map[string]*Stream) {
	p.group.Go(func() error {
		scanner := bufio.NewScanner(readCloser)
		scanner.Buffer(make([]byte, 256*1024), 512*1024)

		for scanner.Scan() {
			if done, err := isCtxDone(ctx); done {
				return err
			}

			bs := scanner.Bytes()

			for name, readCh := range readChs {
				// submit into work pool
				err := p.workerPool.Submit(func() {
					// copy bytes to keep mem safe
					buffer := p.bufferPool.Get()
					defer p.bufferPool.Put(buffer)
					buffer.Reset()

					_, _ = buffer.Write(bs)

					select {
					case <-ctx.Done():
					case <-time.After(time.Second * 20):
					case readCh.ch <- buffer.Bytes():
					}
					return
				})

				if err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			}
		}

		return scanner.Err()
	})
}
