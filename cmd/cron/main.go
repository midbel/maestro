package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/midbel/maestro/schedule"
)

func main() {
	n := flag.Int("n", 5, "show n next scheduled time")
	b := flag.String("b", "", "use base time to start schedule")
	flag.Parse()

	sched, err := schedule.Schedule(flag.Arg(0), flag.Arg(1), flag.Arg(2), flag.Arg(3), flag.Arg(4))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	if *b != "" {
		w, err := time.Parse(time.RFC3339, *b)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		sched.Reset(w)
	}
	for i := 0; i < *n; i++ {
		next := sched.Next()
		fmt.Fprintf(os.Stdout, "%3d) next at %s", i+1, next.Format("2006-01-02 15:04:00"))
		fmt.Fprintln(os.Stdout)
	}
}
