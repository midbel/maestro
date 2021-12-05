package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/maestro/todos"
)

func main() {
	file := flag.String("f", "TODOS", "read TODOS from file")
	flag.Parse()

	r, err := os.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	list, err := todos.Parse(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for i, t := range list {
		fmt.Printf("% 3d | %-8s | %-10s | %-10s | %s", i+1, t.Section, t.Version(), t.State, t.Title)
		fmt.Println()
	}
}
