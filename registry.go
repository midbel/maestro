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

func (r Registry) Lookup(name string) (Executer, error) {
	cmd, ok := r[name]
	if ok {
		return r.prepare(cmd)
	}
	for _, c := range r {
		i := sort.SearchStrings(c.Alias, name)
		if i < len(c.Alias) && c.Alias[i] == name {
			return r.prepare(c)
		}
	}
	return nil, fmt.Errorf("%s: command not defined", name)
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

func (r Registry) prepare(cmd CommandSettings) (Executer, error) {
	exec := command{
		scripts: cmd.Lines,
		workdir: cmd.WorkDir,
	}
	for k, v := range cmd.Ev {
		exec.env = append(exec.env, fmt.Sprintf("%s=%s", k, v))
	}
	return exec, nil
}
