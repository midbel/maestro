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

func copyStringArray(str [][]string, values []string) [][]string {
	if len(str) == 0 {
		for i := range values {
			str = append(str, []string{values[i]})
		}
		return str
	}
	var (
		old  = copyArray(str)
		list [][]string
	)
	for i := range values {
		arr := copyArray(old)
		for j := range arr {
			arr[j] = append(arr[j], values[i])
		}
		list = append(list, arr...)
	}
	return list
}

func copyArray(list [][]string) [][]string {
	var ret [][]string
	for i := range list {
		a := make([]string, len(list[i]))
		copy(a, list[i])
		ret = append(ret, a)
	}
	return ret
}
