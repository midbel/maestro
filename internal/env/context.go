package env

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/midbel/maestro/internal/validate"
)

type Context struct {
	vars  *Env
	flags *flagset
}

func NewContext(name string, vars *Env, args ...validate.ValidateFunc) *Context {
	return &Context{
		vars:  vars.Copy(),
		flags: createFlagset(name, args...),
	}
}

func (e *Context) ParseArgs(args []string) error {
	return e.flags.Parse(args)
}

func (e *Context) Resolve(ident string) ([]string, error) {
	if vs, err := e.resolve(ident); err == nil {
		return vs, nil
	}
	return e.vars.Resolve(ident)
}

func (e *Context) resolve(ident string) ([]string, error) {
	var str string
	switch ident {
	case "0":
		str = e.flags.Name()
	case "#":
		str = strconv.Itoa(e.flags.NArg())
	case "*":
		str = strings.Join(e.flags.Args(), " ")
	case "@":
		return e.flags.Args(), nil
	default:
		if n, err := strconv.Atoi(ident); err == nil && n < flag.NArg() {
			str = flag.Arg(n - 1)
			break
		}
		f := e.flags.Lookup(ident)
		if f == nil {
			return nil, fmt.Errorf("%s: variable not defined", ident)
		}
		str = f.Value.String()
	}
	return []string{str}, nil
}

func (e *Context) Attach(short, long, help, value string, valid validate.ValidateFunc) error {
	if e.isFlag(short) || e.isFlag(long) {
		return fmt.Errorf("%s/%s already set", short, long)
	}
	var (
		str            = stringValue(value)
		val flag.Value = &str
	)
	if valid != nil {
		val = &validValue{
			valid: valid,
			Value: val,
		}
	}
	if short != "" {
		e.flags.Var(val, short, help)
	}
	if long != "" {
		e.flags.Var(val, long, help)
	}
	return nil
}

func (e *Context) AttachFlag(short, long, help string, value bool) error {
	if e.isFlag(short) || e.isFlag(long) {
		return fmt.Errorf("%s/%s already set", short, long)
	}
	val := boolValue(value)
	if short != "" {
		e.flags.Var(&val, short, help)
	}
	if long != "" {
		e.flags.Var(&val, long, help)
	}
	return nil
}

func (e *Context) isFlag(ident string) bool {
	f := e.flags.Lookup(ident)
	return f != nil
}

type flagset struct {
	args []validate.ValidateFunc
	*flag.FlagSet
}

func createFlagset(name string, args ...validate.ValidateFunc) *flagset {
	set := &flagset{
		args:    args,
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError),
	}
	set.FlagSet.Usage = func() {}
	return set
}

func (f *flagset) Parse(args []string) error {
	if err := f.FlagSet.Parse(args); err != nil {
		return err
	}
	for i, validate := range f.args {
		if validate == nil {
			continue
		}
		if err := validate(f.FlagSet.Arg(i)); err != nil {
			return fmt.Errorf("#%d: %s %w", i+1, f.FlagSet.Arg(i), err)
		}
	}
	return nil
}

type boolValue bool

func (v *boolValue) String() string {
	return strconv.FormatBool(bool(*v))
}

func (v *boolValue) Set(str string) error {
	if str == "" {
		*v = boolValue(true)
		return nil
	}
	b, err := strconv.ParseBool(str)
	if err == nil {
		*v = boolValue(b)
	}
	return err
}

type validValue struct {
	valid validate.ValidateFunc
	flag.Value
}

func (v *validValue) Set(str string) error {
	if err := v.valid(str); err != nil {
		return err
	}
	return v.Value.Set(str)
}

type stringValue string

func (v *stringValue) String() string {
	return string(*v)
}

func (v *stringValue) Set(str string) error {
	*v = stringValue(str)
	return nil
}
