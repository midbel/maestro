package maestro

import (
	"fmt"
	"sort"

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
		}
		ctx, err := createContext(cmd)
		if err != nil {
			return nil, err
		}
		r.ctx = ctx
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
		env:     cmd.Ev.All(),
	}
	if !nodeps {
		deps, err := r.resolveDependencies(cmd)
		if err != nil {
			return nil, err
		}
		exec.deps = deps
	}
	ctx, err := createContext(cmd)
	if err != nil {
		return nil, err
	}
	exec.ctx = ctx
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

func createContext(cmd CommandSettings) (*Context, error) {
	ctx := MakeContext(cmd.Name, cmd.locals)
	for _, o := range cmd.Options {
		var err error
		if o.Flag {
			err = ctx.attachFlag(o.Short, o.Long, o.Help, o.DefaultFlag)
		} else {
			err = ctx.attach(o.Short, o.Long, o.Help, o.Default, o.Valid)
		}
		if err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

type canFunc func(CommandSettings) error

func isBlocked(cmd CommandSettings) error {
	if cmd.Blocked() {
		return fmt.Errorf("%s can not be executed", cmd.Name)
	}
	return nil
}
