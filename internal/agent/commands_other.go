//go:build !windows

package agent

import (
	"context"
	"strings"
)

func executeRemoteCommand(ctx context.Context, command remoteCommand) commandResultPayload {
	return commandResultPayload{
		Status: "failed",
		Error:  "comando indisponivel neste sistema: " + strings.TrimSpace(command.Type),
	}
}
