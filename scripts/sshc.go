package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	flag.Parse()

	conf := &ssh.ClientConfig{
		User:            "foobar",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password("foobar"),
		},
	}

	conn, err := ssh.Dial("tcp", flag.Arg(0), conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer sess.Close()

	outr, _ := sess.StdoutPipe()
	errr, _ := sess.StderrPipe()

	go io.Copy(os.Stdout, outr)
	go io.Copy(os.Stderr, errr)

	if err := sess.Run(flag.Arg(1)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}
