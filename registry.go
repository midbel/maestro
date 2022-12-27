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

func (r Registry) Prepare(name string) (Executer, error) {
	cmd, err := r.Lookup(name)
	if err != nil {
		return nil, err
	}
	return cmd.Prepare()
}

func (r Registry) LookupRemote(name string) (CommandSettings, error) {
	cmd, err := r.Lookup(name)
	if err != nil {
		return cmd, err
	}
	if !cmd.Remote() {
		return cmd, fmt.Errorf("%s: command can not be executed on remote server", name)
	}
	return cmd, nil
}

func (r Registry) Lookup(name string) (CommandSettings, error) {
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
