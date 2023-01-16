package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/midbel/shlex"
	"github.com/midbel/slices"
	"golang.org/x/crypto/ssh"
)

const maxAuthTries = 1

func main() {
	var (
		workdir  = flag.String("d", "", "working directory")
		insecure = flag.Bool("i", false, "insecure")
	)
	flag.Parse()

	if *workdir != "" {
		if err := os.Chdir(*workdir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	conf := &ssh.ServerConfig{
		NoClientAuth: *insecure,
		MaxAuthTries: maxAuthTries,
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			return nil, nil
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if sign, err := ssh.NewSignerFromKey(key); err == nil {
		conf.AddHostKey(sign)
	}

	serv, err := net.Listen("tcp", flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer serv.Close()

	for {
		conn, err := serv.Accept()
		if err != nil {
			break
		}
		go handleConn(conn, conf)
	}
}

func handleConn(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	c, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer c.Close()

	go ssh.DiscardRequests(reqs)

	for nch := range chans {
		if kind := nch.ChannelType(); kind != "session" {
			nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("%s: unsupported channel type", kind))
			continue
		}
		ch, reqs, err := nch.Accept()
		if err != nil {
			continue
		}
		go handleRequest(c.User(), c.RemoteAddr(), ch, reqs)
	}
}

func handleRequest(user string, addr net.Addr, ch ssh.Channel, in <-chan *ssh.Request) {
	defer ch.Close()

	for req := range in {
		switch req.Type {
		case "env":
		case "exec":
			execute(user, addr, ch, req)
			return
		default:
			return
		}
	}
}

func execute(who string, addr net.Addr, ch ssh.Channel, req *ssh.Request) {
	parts, err := split(req.Payload)
	if err != nil {
		if req.WantReply {
			req.Reply(true, nil)
		}
		ch.SendRequest("exit-status", false, itob(127))
		return
	}
	var (
		perr *exec.ExitError
		code int
		cmd  = exec.Command(slices.Fst(parts), slices.Rest(parts)...)
	)
	cmd.Stdout = ch
	cmd.Stderr = ch.Stderr()

	now := time.Now()
	if err := cmd.Run(); errors.As(err, &perr) {
		code = perr.ExitCode()
	}
	fmt.Printf("[%s] %s(%s): %s (%s)", now.Format(time.RFC3339), who, addr, strings.Join(parts, " "), time.Since(now))
	fmt.Println()
	if req.WantReply {
		req.Reply(true, nil)
	}
	ch.SendRequest("exit-status", false, itob(code))
}

func split(dat []byte) ([]string, error) {
	dat = bytes.Map(func(r rune) rune {
		if r >= 0x20 && r <= 0x7f {
			return r
		}
		return -1
	}, dat)
	return shlex.Split(bytes.NewReader(dat))
}

func itob(code int) []byte {
	return []byte{
		byte(code >> 24),
		byte(code >> 16),
		byte(code >> 8),
		byte(code),
	}
}
