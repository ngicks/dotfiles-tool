package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/cmd/podman-static-dist/commands"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/cmdsignals"
)

func main() {
	blockOn, ctx, cancel := cmdsignals.NotifyContext(context.Background())

	var wg sync.WaitGroup
	wg.Go(blockOn)

	err := commands.Execute(ctx)

	if err != nil && errors.Is(err, ctx.Err()) {
		if sigErr, ok := errors.AsType[*cmdsignals.SignalReceivedError](context.Cause(ctx)); ok {
			err = sigErr
		}
	}

	cancel(nil)
	wg.Wait()

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
