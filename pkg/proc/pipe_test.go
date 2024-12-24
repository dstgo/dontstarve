package proc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProc_StdoutPipe(t *testing.T) {
	ctx := context.Background()
	proc, err := NewProc(
		ctx,
		WithCommand("echo", "hello world"),
		WithStdout(),
	)
	require.NoError(t, err)

	pipe := proc.StdoutPipe("echo")

	done := make(chan struct{})
	go func() {
		for !pipe.Closed() {
			recv, _ := pipe.Recv()
			fmt.Println(string(recv))
		}
		done <- struct{}{}
		close(done)
	}()

	t.Log(proc.Start())
	time.Sleep(time.Second * 2)
	t.Log(proc.Wait())
	<-done
}

func TestProc_StdinPipe(t *testing.T) {

	sh := `#!/bin/bash

# Loop to read user input
while true; do
    # Prompt the user for input
    echo -n "Please enter something (type 'exit' to quit): "
    read user_input

    # Check if the input is empty
    if [ -z "$user_input" ]; then
        echo "Input cannot be empty, please try again!"
        continue
    fi

    # Check if the input is 'exit' (case-insensitive)
    if [ "$(echo "$user_input" | tr '[:upper:]' '[:lower:]')" == "exit" ]; then
        echo "Exiting script..."
        break
    fi

    # Output the user input
    echo "You entered: $user_input"
done
`

	ctx := context.Background()
	proc, err := NewProc(
		ctx,
		WithCommand("bash", "-c", sh),
		WithStdout(),
		WithStdin(),
	)
	require.NoError(t, err)

	stdoutPipe := proc.StdoutPipe("out")
	stdinPipe := proc.StdinPipe("in")

	stdoutDone := make(chan struct{})
	stdinDone := make(chan struct{})
	go func() {
		for !stdoutPipe.Closed() {
			recv, _ := stdoutPipe.Recv()
			fmt.Println(string(recv))
		}
		stdoutDone <- struct{}{}
		close(stdoutDone)
	}()

	go func() {
		for i := range 10 {
			stdinPipe.Send([]byte{'0' + byte(i), '\n'})
		}
		stdinPipe.Send([]byte("exit\n"))
		stdinDone <- struct{}{}
		close(stdinDone)
	}()

	t.Log(proc.Start())
	time.Sleep(time.Second * 3)
	t.Log(proc.Wait())
	<-stdoutDone
	<-stdinDone
}
