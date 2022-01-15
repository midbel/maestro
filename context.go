package maestro

import (
	"context"
	"os"
	"os/signal"
)

func interruptContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		defer close(sig)
		signal.Notify(sig, os.Kill, os.Interrupt)
		<-sig
		cancel()
	}()
	return ctx
}
