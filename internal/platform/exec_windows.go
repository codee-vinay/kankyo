package platform

import (
	"context"
	"os/exec"
	"syscall"
)

func prepareCommand(ctx context.Context, path string, args []string, batch bool) (*exec.Cmd, string, string, error) {
	if batch {
		systemCmd, err := SystemCmdPath()
		if err != nil {
			return nil, "", "", err
		}
		commandLine, err := BuildBatchCommandLine(systemCmd, path, args)
		if err != nil {
			return nil, "", systemCmd, err
		}
		cmd := exec.CommandContext(ctx, systemCmd)
		cmd.Args = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: commandLine}
		return cmd, commandLine, systemCmd, nil
	}

	return exec.CommandContext(ctx, path, args...), "", "", nil
}
