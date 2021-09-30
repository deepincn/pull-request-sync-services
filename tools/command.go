package tools

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Command struct {
	Dir     string
	Program string
	Args    []string
	Timeout uint
}

func read(ctx context.Context, wg *sync.WaitGroup, std io.ReadCloser) {
	reader := bufio.NewReader(std)
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			readString, err := reader.ReadString('\n')
			if err != nil || err == io.EOF {
				return
			}
			logrus.Info(readString)
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

	var wg sync.WaitGroup
	wg.Add(2)

	go read(ctx, &wg, stderr)
	go read(ctx, &wg, stdout)

	err = c.Start()

	wg.Wait()
	return err
}

func RunCmdList(list []*Command) error {
	for _, command := range list {
		if err := RunSingleCmd(command); err != nil {
			return err
		}
	}
	return nil
}
