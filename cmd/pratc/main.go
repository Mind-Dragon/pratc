package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeffersonnunn/pratc/internal/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd.ExecuteContext(ctx)
}
