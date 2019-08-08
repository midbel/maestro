package maestro

import (
	"golang.org/x/crypto/ssh"
)

type Shell interface {
	Execute(Action) error
}

type local struct{}

func (c *local) Execute(a Action) error {
	return nil
}

type remote struct{
	client *ssh.Client
}

func (r *remote) Execute(a Action) error {
	return nil
}
