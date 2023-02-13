package maestro

import (
	"strings"
	"time"

	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/help"
	"github.com/midbel/maestro/internal/validate"
	"golang.org/x/crypto/ssh"
)

const DefaultSSHPort = 22

type CommandSettings struct {
	Visible bool

	Name       string
	Alias      []string
	Short      string
	Desc       string
	Categories []string

	Retry   int64
	WorkDir string
	Timeout time.Duration

	Hosts   []CommandTarget
	Deps    []CommandDep
	Options []CommandOption
	Args    []CommandArg
	Lines   []CommandScript

	env   *env.Env
	vars  *env.Env
	alias *env.Env
}

func (s CommandSettings) Command() string {
	return s.Name
}

func (s CommandSettings) About() string {
	return s.Short
}

func (s CommandSettings) Help() (string, error) {
	return help.Command(s)
}

func (s CommandSettings) Tags() []string {
	if len(s.Categories) == 0 {
		return []string{"default"}
	}
	return s.Categories
}

func (s CommandSettings) Usage() string {
	var str strings.Builder
	str.WriteString(s.Name)
	for _, o := range s.Options {
		str.WriteString(" ")
		str.WriteString("[")
		if o.Short != "" {
			str.WriteString("-")
			str.WriteString(o.Short)
		}
		if o.Short != "" && o.Long != "" {
			str.WriteString("/")
		}
		if o.Long != "" {
			str.WriteString("--")
			str.WriteString(o.Long)
		}
		str.WriteString("]")
	}
	for _, a := range s.Args {
		str.WriteString(" ")
		str.WriteString("<")
		str.WriteString(a.Name)
		str.WriteString(">")
	}
	return str.String()
}

func (s CommandSettings) Blocked() bool {
	return !s.Visible
}

func (s CommandSettings) Remote() bool {
	return len(s.Hosts) > 0
}

type CommandTarget struct {
	Addr       string
	User       string
	Pass       string
	Key        ssh.Signer
	KnownHosts ssh.HostKeyCallback
}

func createTarget(addr string) CommandTarget {
	return CommandTarget{
		Addr: addr,
	}
}

func (c CommandTarget) Config(top *ssh.ClientConfig) *ssh.ClientConfig {
	conf := &ssh.ClientConfig{
		User:            top.User,
		HostKeyCallback: top.HostKeyCallback,
	}
	if c.KnownHosts != nil {
		conf.HostKeyCallback = c.KnownHosts
	}
	if c.User != "" {
		conf.User = c.User
	}
	if c.Pass != "" {
		conf.Auth = append(conf.Auth, ssh.Password(c.Pass))
	}
	if c.Key != nil {
		conf.Auth = append(conf.Auth, ssh.PublicKeys(c.Key))
	}
	if len(conf.Auth) == 0 {
		conf.Auth = append(conf.Auth, top.Auth...)
	}
	return conf
}

type CommandScript struct {
	Line string
}

func createScript(line string) CommandScript {
	return CommandScript{
		Line: line,
	}
}

type CommandDep struct {
	Name string
	Args []string
}

func createDep(ident string) CommandDep {
	return CommandDep{
		Name: ident,
	}
}

func (c CommandDep) Key() string {
	return c.Name
}

type CommandOption struct {
	Short string
	Long  string
	Help  string
	Flag  bool

	Default     string
	DefaultFlag bool

	Valid validate.ValidateFunc
}

type CommandArg struct {
	Name  string
	Valid validate.ValidateFunc
}

func createArg(name string) CommandArg {
	return CommandArg{
		Name: name,
	}
}

func (a CommandArg) Validate(arg string) error {
	if a.Valid == nil {
		return nil
	}
	return a.Valid(arg)
}
