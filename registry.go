package maestro

import (
	"fmt"
	"sort"
)

type Registry map[string]CommandSettings

func (r Registry) Copy() Registry {
	x := make(Registry)
	for k, v := range r {
		x[k] = v
	}
	return x
}

func (r Registry) Help(name string) (string, error) {
	cmd, ok := r[name]
	if !ok {
		return "", fmt.Errorf("%s: command not defined", name)
	}
	return cmd.Help()
}

func (r Registry) Lookup(name string, nodeps bool) (Executer, error) {
	return r.lookup(name, nodeps, isBlocked)
}

func (r Registry) Exists(name string) bool {
	_, ok := r[name]
	return ok
}

func (r Registry) Register(cmd CommandSettings) error {
	if r.Exists(cmd.Name) {
		return fmt.Errorf("%s: command already registered", cmd.Name)
	}
	r[cmd.Name] = cmd
	return nil
}

func (r Registry) lookup(name string, nodeps bool, can canFunc) (Executer, error) {
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

func (r Registry) find(name string) (CommandSettings, error) {
	cmd, ok := r[name]
	if ok {
		return cmd, nil
	}
	for _, c := range r {
		i := sort.SearchStrings(c.Alias, name)
		if i < len(c.Alias) && c.Alias[i] == name {
			return c, nil
		}
	}
	return cmd, fmt.Errorf("%s: command not defined", name)
}

func (r Registry) prepare(cmd CommandSettings, nodeps bool) (Executer, error) {
	exec := local{
		name:    cmd.Name,
		scripts: cmd.Lines,
		workdir: cmd.WorkDir,
	}
	for k, v := range cmd.Ev {
		exec.env = append(exec.env, fmt.Sprintf("%s=%s", k, v))
	}
	if !nodeps {
		deps, err := r.resolveDependencies(cmd)
		if err != nil {
			return nil, err
		}
		exec.deps = deps
	}
	return exec, nil
}

func (r Registry) resolveDependencies(cmd CommandSettings) ([]Executer, error) {
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

type canFunc func(CommandSettings) error

func isBlocked(cmd CommandSettings) error {
	if cmd.Blocked() {
		return fmt.Errorf("%s can not be executed", cmd.Name)
	}
	return nil
}
