package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

type Command struct {
	Dir     string
	Program string
	Args    []string
	Timeout uint
}

func read(ctx context.Context, std io.ReadCloser, isErr bool) {
	reader := bufio.NewReader(std)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			readString, err := reader.ReadString('\n')
			if err != nil || err == io.EOF {
				return
			}
			if isErr {
				logrus.Error(readString)
			} else {
				logrus.Info(readString)
			}
		}
	}
}

func RunSingleCmd(command *Command) error {
	ctx, cancel := context.WithCancel(context.Background())
	go func(cancelFunc context.CancelFunc) {
		time.Sleep(time.Duration(command.Timeout) * time.Second)
		cancelFunc()
	}(cancel)
	return runSingleCmdByContext(ctx, command)
}

func runSingleCmdByContext(ctx context.Context, command *Command) error {
	c := exec.CommandContext(ctx, command.Program, command.Args...)
	c.Dir = command.Dir

	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := c.StderrPipe()
	if err != nil {
		return err
	}

	go read(ctx, stderr, true)
	go read(ctx, stdout, false)

	err = c.Run()

	if err != nil {
		return err
	}

	if c.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("Exit Code is not 0, %v", command)
	}

	return nil
}

func RunCmdList(list []*Command) error {
	for _, command := range list {
		if err := RunSingleCmd(command); err != nil {
			return err
		}
	}
	return nil
}
