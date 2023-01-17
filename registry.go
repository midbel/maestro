package maestro

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Registry struct {
	builtins map[string]Executer
	commands map[string]CommandSettings
}

func Create() *Registry {
	return &Registry{
		commands: make(map[string]CommandSettings),
	}
}

func (r *Registry) Resolve(name string) (Executer, error) {
	if ex, ok := r.builtins[name]; ok {
		return ex, nil
	}
	return r.Local(name, true)
}

func (r *Registry) Copy() *Registry {
	x := Create()
	for k, v := range r.commands {
		x.commands[k] = v
	}
	return x
}

func (r *Registry) Enable() []CommandSettings {
	var list []CommandSettings
	for _, v := range r.commands {
		if v.Blocked() {
			continue
		}
		list = append(list, v)
	}
	return list
}

func (r *Registry) Graph(name string) ([]string, error) {
	return nil, nil
}

func (r *Registry) Help(name string) (string, error) {
	cmd, ok := r.commands[name]
	if !ok {
		return "", fmt.Errorf("%s: command not defined", name)
	}
	return cmd.Help()
}

func (r *Registry) Local(name string, nodeps bool) (Executer, error) {
	return r.lookup(name, nodeps, isBlocked)
}

func (r *Registry) Remote(name string, config *ssh.ClientConfig) (Executer, error) {
	cmd, err := r.find(name)
	if err != nil {
		return nil, err
	}
	if len(cmd.Hosts) == 0 {
		return nil, fmt.Errorf("%s: no host(s) defined", name)
	}
	var set execset
	for _, h := range cmd.Hosts {
		r := remote{
			name:    name,
			host:    h.Addr,
			scripts: cmd.Lines,
			config:  h.Config(config),
			locals:  cmd.locals.Copy(),
		}
		set.list = append(set.list, r)
	}
	return set, nil
}

func (r *Registry) Exists(name string) bool {
	_, ok := r.commands[name]
	return ok
}

func (r *Registry) Register(cmd CommandSettings) error {
	if r.Exists(cmd.Name) {
		return fmt.Errorf("%s: command already registered", cmd.Name)
	}
	r.commands[cmd.Name] = cmd
	return nil
}

func (r *Registry) lookup(name string, nodeps bool, can canFunc) (Executer, error) {
	cmd, err := r.find(name)
	if err != nil {
		return nil, err
	}
	if can != nil {
		if err := can(cmd); err != nil {
			return nil, err
		}
	}
	ex, err := r.prepare(cmd, nodeps)
	if err != nil {
		return nil, err
	}
	if can != nil {
		ex = Retry(cmd.Retry, ex)
	}
	return ex, nil
}

func (r *Registry) find(name string) (CommandSettings, error) {
	cmd, ok := r.commands[name]
	if ok {
		return cmd, nil
	}
	for _, c := range r.commands {
		i := sort.SearchStrings(c.Alias, name)
		if i < len(c.Alias) && c.Alias[i] == name {
			return c, nil
		}
	}
	return cmd, fmt.Errorf("%s: command not defined", name)
}

func (r *Registry) prepare(cmd CommandSettings, nodeps bool) (Executer, error) {
	exec := local{
		name:    cmd.Name,
		scripts: cmd.Lines,
		workdir: cmd.WorkDir,
		locals:  cmd.locals.Copy(),
		env:     cmd.Ev.All(),
	}
	if !nodeps {
		deps, err := r.resolveDependencies(cmd)
		if err != nil {
			return nil, err
		}
		exec.deps = deps
	}
	set, err := prepareArgs(cmd)
	if err != nil {
		return nil, err
	}
	exec.flagset = set
	return exec, nil
}

func (r *Registry) resolveDependencies(cmd CommandSettings) ([]Executer, error) {
	var (
		seen = make(map[string]struct{})
		list []Executer
	)
	for _, d := range cmd.Deps {
		if _, ok := seen[d.Name]; ok {
			continue
		}
		seen[d.Name] = struct{}{}

		ex, err := r.lookup(d.Name, false, nil)
		if err != nil {
			return nil, err
		}
		list = append(list, ex)
	}
	return list, nil
}

func prepareArgs(cmd CommandSettings) (*flag.FlagSet, error) {
	var (
		set   = flag.NewFlagSet(cmd.Name, flag.ExitOnError)
		seen  = make(map[string]struct{})
		empty = struct{}{}
	)
	set.Usage = func() {
		help, err := cmd.Help()
		if err != nil {
			return
		}
		fmt.Fprintln(set.Output(), strings.TrimSpace(help))
		os.Exit(1)
	}
	check := func(name string) error {
		if name == "" {
			return nil
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("%s: option already defined", name)
		}
		seen[name] = empty
		return nil
	}
	attach := func(name, help, value string, target *string) error {
		err := check(name)
		if err == nil {
			set.StringVar(target, name, value, help)
		}
		return err
	}
	attachFlag := func(name, help string, value bool, target *bool) error {
		err := check(name)
		if err == nil {
			set.BoolVar(target, name, value, help)
		}
		return err
	}
	for i, o := range cmd.Options {
		var e1, e2 error
		if o.Flag {
			e1 = attachFlag(o.Short, o.Help, o.DefaultFlag, &cmd.Options[i].TargetFlag)
			e2 = attachFlag(o.Long, o.Help, o.DefaultFlag, &cmd.Options[i].TargetFlag)
		} else {
			e1 = attach(o.Short, o.Help, o.Default, &cmd.Options[i].Target)
			e2 = attach(o.Long, o.Help, o.Default, &cmd.Options[i].Target)
		}
		if err := hasError(e1, e2); err != nil {
			return nil, err
		}
	}
	return set, nil
}

type canFunc func(CommandSettings) error

func isBlocked(cmd CommandSettings) error {
	if cmd.Blocked() {
		return fmt.Errorf("%s can not be executed", cmd.Name)
	}
	return nil
}
