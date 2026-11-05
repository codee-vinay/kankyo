package platform

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

const (
	StdoutLimit = 32 << 10
	StderrLimit = 32 << 10
)

type RunResult struct {
	Stdout       string
	Stderr       string
	ExitCode     int
	TimedOut     bool
	ErrorCode    string
	ErrorMessage string
	CommandLine  string
	Interpreter  string
	Duration     time.Duration
}

// Run executes one fixed, allowlisted command with a bounded timeout and
// bounded output retention. The platform-specific prepareCommand decides how a
// batch file is launched and how the process is terminated.
func Run(ctx context.Context, path string, args []string, batch bool, timeout time.Duration) RunResult {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd, commandLine, interpreter, err := prepareCommand(runCtx, path, args, batch)
	if err != nil {
		return RunResult{
			ExitCode:     -1,
			ErrorCode:    "prepare",
			ErrorMessage: err.Error(),
			Interpreter:  interpreter,
		}
	}

	stdout := newCappedBuffer(StdoutLimit)
	stderr := newCappedBuffer(StderrLimit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = nil
	cmd.WaitDelay = 500 * time.Millisecond

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	result := RunResult{
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		ExitCode:    -1,
		CommandLine: commandLine,
		Interpreter: interpreter,
		Duration:    elapsed,
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	switch {
	case errors.Is(runErr, exec.ErrWaitDelay):
		result.TimedOut = true
		result.ErrorCode = "output_drain_timeout"
		result.ErrorMessage = runErr.Error()
	case runCtx.Err() == context.DeadlineExceeded && runErr != nil:
		result.TimedOut = true
		result.ErrorCode = "timeout"
		result.ErrorMessage = "version check exceeded its timeout"
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			result.ErrorCode = "exit"
		} else {
			result.ErrorCode = "start"
		}
		result.ErrorMessage = runErr.Error()
	}

	return result
}
