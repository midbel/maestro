package maestro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/maestro/internal/copyslice"
	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/schedule"
	"github.com/midbel/shlex"
	"github.com/midbel/tish"
	"golang.org/x/crypto/ssh"
)

const (
	metaNamespace  = "NAMESPACE"
	metaWorkDir    = "WORKDIR"
	metaTrace      = "TRACE"
	metaAll        = "ALL"
	metaDefault    = "DEFAULT"
	metaBefore     = "BEFORE"
	metaAfter      = "AFTER"
	metaError      = "ERROR"
	metaSuccess    = "SUCCESS"
	metaAuthor     = "AUTHOR"
	metaEmail      = "EMAIL"
	metaVersion    = "VERSION"
	metaUsage      = "USAGE"
	metaHelp       = "HELP"
	metaUser       = "SSH_USER"
	metaPass       = "SSH_PASSWORD"
	metaPubKey     = "SSH_PUBKEY"
	metaKnownHosts = "SSH_KNOWN_HOSTS"
	metaParallel   = "SSH_PARALLEL"
	metaCertFile   = "HTTP_CERT_FILE"
	metaKeyFile    = "HTTP_CERT_KEY"
)

const (
	propHelp     = "help"
	propShort    = "short"
	propTags     = "tag"
	propRetry    = "retry"
	propWorkDir  = "workdir"
	propTimeout  = "timeout"
	propHosts    = "hosts"
	propOpts     = "options"
	propArg      = "args"
	propAlias    = "alias"
	propSchedule = "schedule"
)

const (
	schedTime              = "time"
	schedOverlap           = "overlap"
	schedNotify            = "notify"
	schedArgs              = "args"
	schedEnv               = "env"
	schedOut               = "stdout"
	schedErr               = "stderr"
	schedRedirectFile      = "file"
	schedRedirectCompress  = "compress"
	schedRedirectDuplicate = "duplicate"
	schedRedirectOverwrite = "overwrite"
)

const (
	optShort    = "short"
	optLong     = "long"
	optRequired = "required"
	optDefault  = "default"
	optFlag     = "flag"
	optHelp     = "help"
	optValid    = "check"
)

type Decoder struct {
	locals *env.Env
	env    map[string]string
	alias  map[string]string
	frames []*frame
}

func Decode(r io.Reader) (*Maestro, error) {
	d, err := NewDecoder(r)
	if err != nil {
		return nil, err
	}
	return d.Decode()
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	return NewDecoderWithEnv(r, env.EmptyEnv())
}

func NewDecoderWithEnv(r io.Reader, ev *env.Env) (*Decoder, error) {
	if ev == nil {
		ev = env.EmptyEnv()
	}
	d := Decoder{
		locals: ev,
		env:    make(map[string]string),
		alias:  make(map[string]string),
	}
	if err := d.push(r); err != nil {
		return nil, err
	}
	return &d, nil
}

func (d *Decoder) Decode() (*Maestro, error) {
	mst := New()
	return mst, d.decode(mst)
}

func (d *Decoder) decode(mst *Maestro) error {
	d.skipNL()
	for !d.done() {
		var err error
		switch d.curr().Type {
		case Ident:
			if d.peek().IsAssign() {
				err = d.decodeVariable()
				break
			}
			err = d.decodeCommand(mst)
		case Hidden:
			err = d.decodeCommand(mst)
		case Meta:
			err = d.decodeMeta(mst)
		case Keyword:
			err = d.decodeKeyword(mst)
		case Comment:
			d.next()
		default:
			err = d.unexpected()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeKeyword(mst *Maestro) error {
	var err error
	switch d.curr().Literal {
	case kwInclude:
		err = d.decodeInclude(mst)
	case kwExport:
		err = d.decodeExport(mst)
	case kwDelete:
		err = d.decodeDelete(mst)
	case kwAlias:
		err = d.decodeAlias(mst)
	default:
		err = d.unexpected()
	}
	return err
}

func (d *Decoder) decodeInclude(mst *Maestro) error {
	type include struct {
		file     string
		optional bool
	}
	decode := func() (include, error) {
		var (
			str []string
			inc include
		)
		for !d.done() && d.curr().IsValue() {
			vs, err := d.decodeValue()
			if err != nil {
				return inc, err
			}
			str = append(str, vs...)
		}
		inc.file = strings.Join(str, "")
		if d.curr().Type == Optional {
			inc.optional = true
			d.next()
		}
		return inc, d.ensureEOL()
	}
	d.next()
	var list []include
	switch curr := d.curr(); {
	case curr.IsValue():
		i, err := decode()
		if err != nil {
			return err
		}
		list = append(list, i)
	case curr.Type == BegList:
		d.next()
		if err := d.ensureEOL(); err != nil {
			return err
		}
		for !d.done() && d.curr().Type != EndList {
			i, err := decode()
			if err != nil {
				return err
			}
			list = append(list, i)
		}
		if d.curr().Type != EndList {
			return d.unexpected()
		}
		d.next()
		if err := d.ensureEOL(); err != nil {
			return err
		}
	default:
		return d.unexpected()
	}
	for i := range list {
		file, ok := mst.Includes.Exists(list[i].file)
		if !ok {
			if list[i].optional {
				continue
			}
			return fmt.Errorf("%s: file does not exists in %s", file, mst.Includes)
		}
		if err := d.decodeFile(file); err != nil {
			if list[i].optional {
				continue
			}
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeFile(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()
	return d.push(r)
}

func (d *Decoder) decodeExport(msg *Maestro) error {
	decode := func() error {
		ident := d.curr()
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		if !d.curr().IsValue() {
			return d.unexpected()
		}
		if d.curr().IsVariable() {
			vs, err := d.locals.Resolve(d.curr().Literal)
			if err != nil {
				return err
			}
			if len(vs) > 0 {
				d.env[ident.Literal] = vs[0]
			}
		} else {
			d.env[ident.Literal] = d.curr().Literal
		}
		d.next()
		d.skipBlank()
		return d.ensureEOL()
	}
	d.next()
	switch d.curr().Type {
	case Ident:
		if err := decode(); err != nil {
			return err
		}
	case BegList:
		d.next()
		if err := d.ensureEOL(); err != nil {
			return err
		}
		for !d.done() && d.curr().Type != EndList {
			if err := decode(); err != nil {
				return err
			}
		}
		if d.curr().Type != EndList {
			return d.unexpected()
		}
		d.next()
	default:
		return d.unexpected()
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeDelete(mst *Maestro) error {
	d.next()
	for !d.done() && !d.curr().IsEOL() {
		if !d.curr().IsValue() {
			return d.unexpected()
		}
		d.locals.Delete(d.curr().Literal)
		d.next()
		switch d.curr().Type {
		case Ident, Eol:
		default:
			return d.unexpected()
		}
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeAlias(mst *Maestro) error {
	decode := func() error {
		var (
			ident = d.curr()
			str   []string
		)
		d.next()
		if !d.curr().IsAssign() {
			return d.unexpected()
		}
		d.next()
		for !d.done() {
			vs, err := d.decodeValue()
			if err != nil {
				return err
			}
			str = append(str, vs...)
			if !d.curr().IsBlank() {
				break
			}
			d.skipBlank()
		}
		d.alias[ident.Literal] = strings.Join(str, " ")
		return d.ensureEOL()
	}
	d.next()
	switch d.curr().Type {
	case Ident:
		return decode()
	case BegList:
		d.next()
		if err := d.ensureEOL(); err != nil {
			return err
		}
		for !d.done() && d.curr().Type != EndList {
			if err := decode(); err != nil {
				return err
			}
		}
		if d.curr().Type != EndList {
			return d.unexpected()
		}
		d.next()
		return d.ensureEOL()
	default:
		return d.unexpected()
	}
}

func (d *Decoder) decodeObjectVariable(ident string) error {
	d.locals = env.EnclosedEnv(d.locals)
	err := d.decodeObject(d.decodeAssignment)
	if err != nil {
		return err
	}
	// restore the original env
	d.locals = d.locals.Unwrap()
	return nil
}

func (d *Decoder) decodeObject(decode func() error) error {
	d.next()
	d.skipNL()
	var done bool
	for !d.done() && !done {
		d.skipComment()
		if err := decode(); err != nil {
			return err
		}
		switch d.curr().Type {
		case Ident, String:
		case Comment:
			d.next()
		case Comma:
			d.next()
			d.skipComment()
			d.skipNL()
		case Eol:
			d.skipNL()
		case EndList:
		default:
			return d.unexpected()
		}
		done = d.curr().Type == EndList
	}
	if d.curr().Type != EndList {
		return d.unexpected()
	}
	d.next()
	return nil
}

func (d *Decoder) decodeAssignment() error {
	var (
		ident  = d.curr()
		assign bool
	)
	d.next()
	if !d.curr().IsAssign() {
		return d.unexpected()
	}
	assign = d.curr().Type == Assign
	d.next()

	if d.curr().Type == BegList {
		if !assign {
			return d.unexpected()
		}
		return d.decodeObjectVariable(ident.Literal)
	}

	var str []string
	for !d.done() {
		xs, err := d.decodeValue()
		if err != nil {
			return err
		}
		str = append(str, xs...)
		if !d.curr().IsBlank() {
			break
		}
		d.skipBlank()
	}
	if assign {
		d.locals.Define(ident.Literal, str)
	} else {
		xs, _ := d.locals.Resolve(ident.Literal)
		d.locals.Define(ident.Literal, append(xs, str...))
	}
	return nil
}

func (d *Decoder) decodeVariable() error {
	if err := d.decodeAssignment(); err != nil {
		return err
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeScript(line string) ([]string, error) {
	var (
		buf  bytes.Buffer
		opts = []tish.ShellOption{
			tish.WithEnv(d.locals),
			tish.WithStdout(&buf),
		}
		sh, _ = tish.New(opts...)
	)
	if err := sh.Execute(context.TODO(), line, "", nil); err != nil {
		return nil, err
	}
	return shlex.Split(&buf)
}

func (d *Decoder) decodeCommand(mst *Maestro) error {
	var hidden bool
	if hidden = d.curr().Type == Hidden; hidden {
		d.next()
	}
	cmd, err := NewCommandSettingsWithLocals(d.curr().Literal, d.locals)
	if err != nil {
		return err
	}
	cmd.Ev = copyslice.CopyMap[string, string](d.env)
	cmd.As = copyslice.CopyMap[string, string](d.alias)
	cmd.Visible = !hidden
	d.next()
	if d.curr().Type == BegList {
		if err := d.decodeCommandProperties(&cmd); err != nil {
			return err
		}
	}
	if d.curr().Type == Dependency {
		if err := d.decodeCommandDependencies(&cmd); err != nil {
			return err
		}
	}
	if d.curr().Type == BegScript {
		if err := d.decodeCommandScripts(&cmd, mst); err != nil {
			return err
		}
	}
	if err := mst.Register(cmd); err != nil {
		return err
	}
	return nil
}

func (d *Decoder) decodeCommandProperties(cmd *CommandSettings) error {
	return d.decodeObject(func() error {
		var (
			curr = d.curr()
			err  error
		)
		switch {
		case curr.Type == Ident:
		case curr.Type == Keyword && curr.Literal == kwAlias:
		default:
			return d.unexpected()
		}
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		switch curr.Literal {
		default:
			err = fmt.Errorf("%s: unknown command property", curr.Literal)
		case propShort:
			cmd.Short, err = d.parseString()
		case propHelp:
			cmd.Desc, err = d.parseString()
		case propTags:
			cmd.Categories, err = d.parseStringList()
		case propRetry:
			cmd.Retry, err = d.parseInt()
		case propTimeout:
			cmd.Timeout, err = d.parseDuration()
		case propHosts:
			cmd.Hosts, err = d.parseStringList()
			sort.Strings(cmd.Hosts)
		case propAlias:
			cmd.Alias, err = d.parseStringList()
			sort.Strings(cmd.Alias)
		case propArg:
			cmd.Args, err = d.decodeCommandArguments()
		case propOpts:
			err = d.decodeCommandOptions(cmd)
		case propSchedule:
			err = d.decodeCommandSchedule(cmd)
		}
		return err
	})
}

func (d *Decoder) decodeCommandSchedule(cmd *CommandSettings) error {
	var done bool
	for !d.done() && !done {
		if t := d.curr().Type; t != BegList {
			if t == Ident || t == String {
				return nil
			}
			return d.unexpected()
		}
		sched, err := d.decodeScheduleObject()
		if err != nil {
			return err
		}
		cmd.Schedules = append(cmd.Schedules, sched)
		switch d.curr().Type {
		case Comma:
			d.next()
			d.skipComment()
			d.skipNL()
		case Eol:
			d.skipNL()
		case EndList:
		default:
			return d.unexpected()
		}
		done = d.curr().Type == EndList
	}
	if d.curr().Type != EndList {
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) decodeScheduleObject() (Schedule, error) {
	var (
		sched Schedule
		err   error
	)
	err = d.decodeObject(func() error {
		var (
			curr = d.curr()
			err  error
		)
		if curr.Type != Ident {
			return d.unexpected()
		}
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		switch curr.Literal {
		default:
			return fmt.Errorf("%s: unknown schedule property", curr.Literal)
		case schedTime:
			sched.Sched, err = d.parseCrontab()
		case schedOverlap:
			sched.Overlap, err = d.parseBool()
		case schedNotify:
			sched.Notify, err = d.parseStringList()
		case schedArgs:
			sched.Args, err = d.parseStringList()
		case schedEnv:
			// TODO
		case schedOut:
			sched.Stdout, err = d.decodeScheduleRedirect()
		case schedErr:
			sched.Stderr, err = d.decodeScheduleRedirect()
		}
		return err
	})
	return sched, err
}

func (d *Decoder) decodeScheduleRedirect() (ScheduleRedirect, error) {
	var (
		redirect ScheduleRedirect
		err      error
	)
	switch d.curr().Type {
	case Ident, String:
		redirect.File, err = d.parseString()
		return redirect, err
	case BegList:
	default:
		return redirect, d.unexpected()
	}
	err = d.decodeObject(func() error {
		var (
			curr = d.curr()
			err  error
		)
		if curr.Type != Ident {
			return d.unexpected()
		}
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		switch curr.Literal {
		default:
			return fmt.Errorf("%s: unknown schedule property", curr.Literal)
		case schedRedirectFile:
			redirect.File, err = d.parseString()
		case schedRedirectCompress:
			redirect.Compress, err = d.parseBool()
		case schedRedirectDuplicate:
			redirect.Duplicate, err = d.parseBool()
		case schedRedirectOverwrite:
			redirect.Overwrite, err = d.parseBool()
		}
		return err
	})
	return redirect, err
}

func (d *Decoder) decodeCommandArguments() ([]CommandArg, error) {
	var args []CommandArg
	for !d.done() && d.curr().Type != Comma {
		if d.curr().Type != Ident {
			return nil, d.unexpected()
		}
		arg := CommandArg{
			Name: d.curr().Literal,
		}
		d.next()
		d.skipBlank()
		if d.curr().Type == BegList {
			d.next()
			list, err := d.decodeValidationRules(EndList)
			if err != nil {
				return nil, err
			}
			switch len(list) {
			case 0:
			case 1:
				arg.Valid = list[0]
			default:
				arg.Valid = validateAll(list...)
			}
		}
		args = append(args, arg)
	}
	if d.curr().Type != Comma {
		return nil, d.unexpected()
	}
	return args, nil
}

func (d *Decoder) decodeOptionObject() (CommandOption, error) {
	var opt CommandOption
	return opt, d.decodeObject(func() error {
		var (
			curr = d.curr()
			err  error
		)
		if curr.Type != Ident {
			return d.unexpected()
		}
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		switch curr.Literal {
		default:
			return fmt.Errorf("%s: unknown option property", curr.Literal)
		case optShort:
			opt.Short, err = d.parseString()
		case optLong:
			opt.Long, err = d.parseString()
		case optDefault:
			opt.Default, err = d.parseString()
		case optRequired:
			opt.Required, err = d.parseBool()
		case optFlag:
			opt.Flag, err = d.parseBool()
		case optHelp:
			opt.Help, err = d.parseString()
		case optValid:
			opt.Valid, err = d.decodeBasicValidateOption()
		}
		return err
	})
}

func (d *Decoder) decodeCommandOptions(cmd *CommandSettings) error {
	var done bool
	for !d.done() && !done {
		if t := d.curr().Type; t != BegList {
			if t == Ident || t == String {
				return nil
			}
			return d.unexpected()
		}
		opt, err := d.decodeOptionObject()
		if err != nil {
			return err
		}
		cmd.Options = append(cmd.Options, opt)
		switch d.curr().Type {
		case Comma:
			d.next()
			d.skipComment()
			d.skipNL()
		case Eol:
			d.skipNL()
		case EndList:
		default:
			return d.unexpected()
		}
		done = d.curr().Type == EndList
	}
	if d.curr().Type != EndList {
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) decodeSpecialValidateOption(rule string) (ValidateFunc, error) {
	if d.curr().Type != BegList {
		return nil, d.unexpected()
	}
	d.next()
	list, err := d.decodeValidationRules(EndList)
	if err != nil {
		return nil, err
	}
	var fn ValidateFunc
	switch rule {
	case validNot:
		fn = validateError(validateAll(list...))
	case validSome:
		fn = validateSome(list...)
	case validAll:
		fn = validateAll(list...)
	default:
		// should never happens
		return nil, fmt.Errorf("%s: unknown validation function", rule)
	}
	return fn, nil
}

func (d *Decoder) decodeBasicValidateOption() (ValidateFunc, error) {
	list, err := d.decodeValidationRules(Comma)
	if err != nil {
		return nil, err
	}
	switch len(list) {
	case 0:
		return nil, fmt.Errorf("%s is given but rules are supplied", optValid)
	case 1:
		return list[0], nil
	default:
		return validateAll(list...), nil
	}
}

func (d *Decoder) decodeValidationRules(until rune) ([]ValidateFunc, error) {
	var list []ValidateFunc
	for !d.done() && d.curr().Type != until {
		if d.curr().Type != Ident {
			return nil, d.unexpected()
		}
		var (
			rule = d.curr().Literal
			args []string
		)
		d.next()
		d.skipBlank()
		if rule == validNot || rule == validSome || rule == validAll {
			fn, err := d.decodeSpecialValidateOption(rule)
			if err != nil {
				return nil, err
			}
			list = append(list, fn)
			continue
		}
		if d.curr().Type == BegList {
			d.next()
			for !d.done() && d.curr().Type != EndList {
				switch curr := d.curr(); {
				case curr.IsPrimitive():
					args = append(args, curr.Literal)
				case curr.IsVariable():
					vs, err := d.locals.Resolve(curr.Literal)
					if err != nil {
						return nil, err
					}
					args = append(args, vs...)
				default:
					return nil, d.unexpected()
				}
				d.next()
				d.skipBlank()
			}
			if d.curr().Type != EndList {
				return nil, d.unexpected()
			}
			d.next()
			d.skipBlank()
		}
		fn, err := getValidateFunc(rule, args)
		if err != nil {
			return nil, err
		}
		list = append(list, fn)
	}
	if d.curr().Type != until {
		return nil, d.unexpected()
	}
	d.next()
	return list, nil
}

func (d *Decoder) decodeCommandDependencies(cmd *CommandSettings) error {
	d.next()
	for !d.done() {
		if d.curr().Type == BegScript {
			break
		}
		var optional, mandatory, space bool
		for d.curr().Type != Ident {
			switch d.curr().Type {
			case Mandatory:
				mandatory = true
			case Optional:
				optional = true
			default:
				return d.unexpected()
			}
			d.next()
		}
		switch d.curr().Type {
		case Resolution:
			space = true
			d.next()
		case Ident:
		default:
			return d.unexpected()
		}
		dep := CommandDep{
			Name:      d.curr().Literal,
			Optional:  optional,
			Mandatory: mandatory,
		}
		d.next()
		if d.curr().Type == Resolution {
			if space {
				return d.unexpected()
			}
			d.next()
			if d.curr().Type != Ident {
				return d.unexpected()
			}
			dep.Space = dep.Name
			dep.Name = d.curr().Literal
			d.next()
		}
		if d.curr().Type == BegList {
			d.next()
			for !d.done() && d.curr().Type != EndList {
				switch curr := d.curr(); {
				case curr.IsPrimitive():
					dep.Args = append(dep.Args, curr.Literal)
				case curr.IsVariable():
					vs, err := d.locals.Resolve(curr.Literal)
					if err != nil {
						return err
					}
					dep.Args = append(dep.Args, vs...)
				default:
					return d.unexpected()
				}
				d.next()
				if d.curr().Type == Comma {
					d.next()
				}
			}
			if d.curr().Type != EndList {
				return d.unexpected()
			}
			d.next()
		}
		if d.curr().Type == Background {
			dep.Bg = true
			d.next()
		}
		cmd.Deps = append(cmd.Deps, dep)
		switch d.curr().Type {
		case Comma:
			d.next()
		case BegScript:
		default:
			return d.unexpected()
		}
	}
	if d.curr().Type != BegScript {
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) decodeCommandHelp(cmd *CommandSettings) error {
	var (
		help strings.Builder
		prev string
	)
	for i := 0; !d.done() && d.curr().Type == Comment; i++ {
		str := d.curr().Literal
		if str == "" && prev == "" {
			d.next()
			continue
		}
		help.WriteString(strings.TrimSpace(str))
		help.WriteString("\n")
		prev = str
		d.next()
	}
	cmd.Desc = strings.TrimSpace(help.String())
	return nil
}

func (d *Decoder) decodeCommandScripts(cmd *CommandSettings, mst *Maestro) error {
	d.next()
	if err := d.decodeCommandHelp(cmd); err != nil {
		return err
	}
	for !d.done() && d.curr().Type != EndScript {
		var err error
		switch d.curr().Type {
		case Comment:
			d.next()
		default:
			line, err1 := d.decodeScriptLine()
			if err1 != nil {
				err = err1
				break
			}
			cmd.Lines = append(cmd.Lines, line)
		}
		if err != nil {
			return err
		}
	}
	if d.curr().Type != EndScript {
		return d.unexpected()
	}
	d.next()
	return d.ensureEOL()
}

func (d *Decoder) decodeScriptLine() (string, error) {
	if d.curr().Type != Script {
		return "", d.unexpected()
	}
	defer d.next()
	return d.curr().Literal, nil
}

func (d *Decoder) decodeMeta(mst *Maestro) error {
	var (
		meta = d.curr()
		err  error
	)
	d.next()
	if d.curr().Type != Assign {
		return d.unexpected()
	}
	d.next()
	switch meta.Literal {
	case metaNamespace:
		mst.MetaExec.Namespace, err = d.parseString()
	case metaWorkDir:
		mst.MetaExec.WorkDir, err = d.parseString()
	case metaTrace:
		mst.MetaExec.Trace, err = d.parseBool()
	case metaAll:
		mst.MetaExec.All, err = d.parseStringList()
	case metaDefault:
		mst.MetaExec.Default, err = d.parseString()
	case metaBefore:
		mst.MetaExec.Before, err = d.parseStringList()
	case metaAfter:
		mst.MetaExec.After, err = d.parseStringList()
	case metaError:
		mst.MetaExec.Error, err = d.parseStringList()
	case metaSuccess:
		mst.MetaExec.Success, err = d.parseStringList()
	case metaAuthor:
		mst.MetaAbout.Author, err = d.parseString()
	case metaEmail:
		mst.MetaAbout.Email, err = d.parseString()
	case metaVersion:
		mst.MetaAbout.Version, err = d.parseString()
	case metaUsage:
		mst.MetaAbout.Usage, err = d.parseString()
	case metaHelp:
		mst.MetaAbout.Help, err = d.parseString()
	case metaUser:
		mst.MetaSSH.User, err = d.parseString()
	case metaPass:
		mst.MetaSSH.Pass, err = d.parseString()
	case metaPubKey:
		mst.MetaSSH.Key, err = d.parseSignerSSH()
	case metaKnownHosts:
		mst.MetaSSH.Hosts, err = d.parseKnownHosts()
	case metaParallel:
		mst.MetaSSH.Parallel, err = d.parseInt()
	case metaCertFile:
		mst.MetaHttp.CertFile, err = d.parseString()
	case metaKeyFile:
		mst.MetaHttp.KeyFile, err = d.parseString()
	default:
		return fmt.Errorf("%s: unknown/unsupported meta", meta)
	}
	if err == nil {
		err = d.ensureEOL()
	}
	return err
}

func (d *Decoder) ensureNext(list ...rune) error {
	for i := range list {
		d.next()
		if list[i] != d.curr().Type {
			return d.unexpected()
		}
	}
	return nil
}

func (d *Decoder) ensureEOL() error {
	switch d.curr().Type {
	case Eol, Comment:
		d.next()
	default:
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) decodeQuote() (string, error) {
	d.next()
	var str []string
	for !d.done() && d.curr().Type != Quote {
		if d.curr().IsVariable() {
			vs, err := d.locals.Resolve(d.curr().Literal)
			if err != nil {
				return "", err
			}
			if len(vs) != 1 {
				return "", fmt.Errorf("quote: too many values")
			}
			str = append(str, vs[0])
		} else {
			str = append(str, d.curr().Literal)
		}
		d.next()
	}
	if d.curr().Type != Quote {
		return "", d.unexpected()
	}
	return strings.Join(str, ""), nil
}

func (d *Decoder) decodeValue() ([]string, error) {
	var str [][]string
	for d.curr().IsValue() {
		var tmp []string
		switch curr := d.curr(); {
		case curr.IsVariable():
			vs, err := d.locals.Resolve(d.curr().Literal)
			if err != nil {
				return nil, err
			}
			tmp = vs
		case curr.Type == Quote:
			s, err := d.decodeQuote()
			if err != nil {
				return nil, err
			}
			tmp = append(tmp, s)
		default:
			tmp = append(tmp, d.curr().Literal)
		}
		d.next()
		str = copyslice.CopyValues[string](str, tmp)
	}
	ret := make([]string, len(str))
	for i := range str {
		ret[i] = strings.Join(str[i], "")
	}
	return ret, nil
}

func (d *Decoder) parseStringList() ([]string, error) {
	if d.curr().Type == Eol || d.curr().Type == Comment {
		return nil, nil
	}
	var str []string
	for !d.done() {
		xs, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		str = append(str, xs...)
		if !d.curr().IsBlank() {
			break
		}
		d.skipBlank()
	}
	return str, nil
}

func (d *Decoder) parseString() (string, error) {
	if d.curr().Type == Eol || d.curr().Type == Comment {
		return "", nil
	}
	if !d.curr().IsValue() {
		return "", d.unexpected()
	}
	str, err := d.decodeValue()
	if err != nil {
		return "", err
	}
	if len(str) != 1 {
		return "", fmt.Errorf("too many values")
	}
	return str[0], nil
}

func (d *Decoder) parseCrontab() (*schedule.Scheduler, error) {
	list, err := d.parseStringList()
	if err != nil {
		return nil, err
	}
	return schedule.ScheduleFromList(list)
}

func (d *Decoder) parseKnownHosts() ([]hostEntry, error) {
	file, err := d.parseString()
	if err != nil {
		return nil, err
	}
	if file == "default" || file == "" {
		file = defaultKnownHost
	}
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var list []hostEntry
	for len(buf) > 0 {
		_, hosts, key, _, rest, err := ssh.ParseKnownHosts(buf)
		if err != nil {
			return nil, err
		}
		for i := range hosts {
			list = append(list, createEntry(hosts[i], key))
		}
		buf = rest
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Host < list[j].Host
	})
	return list, nil
}

func (d *Decoder) parseSignerSSH() (ssh.Signer, error) {
	file, err := d.parseString()
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(buf)
}

func (d *Decoder) parseBool() (bool, error) {
	str, err := d.parseString()
	if err != nil || str == "" {
		return false, err
	}
	return strconv.ParseBool(str)
}

func (d *Decoder) parseInt() (int64, error) {
	str, err := d.parseString()
	if err != nil || str == "" {
		return 0, err
	}
	return strconv.ParseInt(str, 0, 64)
}

func (d *Decoder) parseDuration() (time.Duration, error) {
	str, err := d.parseString()
	if err != nil || str == "" {
		return 0, err
	}
	return time.ParseDuration(str)
}

func (d *Decoder) skipBlank() {
	d.skip(Blank)
}

func (d *Decoder) skipNL() {
	d.skip(Eol)
}

func (d *Decoder) skipComment() {
	d.skip(Comment)
}

func (d *Decoder) skip(kind rune) {
	for d.curr().Type == kind {
		d.next()
	}
}

func (d *Decoder) next() {
	z := len(d.frames)
	if z == 0 {
		return
	}
	z--
	d.frames[z].next()
	if d.frames[z].done() {
		d.pop()
		z--
	}
	if z < 0 {
		return
	}
}

func (d *Decoder) done() bool {
	z := len(d.frames)
	if z == 1 {
		return d.frames[0].done()
	}
	return false
}

func (d *Decoder) unexpected() error {
	return unexpected(d.curr(), d.CurrentLine())
}

func (d *Decoder) undefined() error {
	return fmt.Errorf("maestro: %s: %w", d.curr().Literal, errUndefined)
}

func (d *Decoder) push(r io.Reader) error {
	f, err := makeFrame(r)
	if err != nil {
		return err
	}
	d.frames = append(d.frames, f)
	d.locals = env.EnclosedEnv(d.locals)
	return nil
}

func (d *Decoder) pop() error {
	z := len(d.frames)
	if z <= 1 {
		return nil
	}
	z--
	d.frames = d.frames[:z]
	d.locals = d.locals.Unwrap()
	return nil
}

func (d *Decoder) curr() Token {
	var t Token
	if z := len(d.frames); z > 0 {
		t = d.frames[z-1].curr
	}
	return t
}

func (d *Decoder) peek() Token {
	var t Token
	if z := len(d.frames); z > 0 {
		t = d.frames[z-1].peek
	}
	return t
}

func (d *Decoder) CurrentLine() string {
	z := len(d.frames)
	if z == 0 {
		return ""
	}
	return d.frames[z-1].scan.CurrentLine()
}

var (
	errUnexpected = errors.New("unexpected token")
	errUndefined  = errors.New("undefined variable")
)

type frame struct {
	curr Token
	peek Token
	scan *Scanner
}

func makeFrame(r io.Reader) (*frame, error) {
	s, err := Scan(r)
	if err != nil {
		return nil, err
	}
	f := frame{
		scan: s,
	}
	f.next()
	f.next()
	return &f, nil
}

func createFrame(file string) (*frame, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return makeFrame(r)
}

func (f *frame) Line() string {
	return f.scan.CurrentLine()
}

func (f *frame) next() {
	f.curr = f.peek
	f.peek = f.scan.Scan()
}

func (f *frame) done() bool {
	return f.curr.IsEOF()
}

type UnexpectedError struct {
	Line     string
	Invalid  Token
	Expected []string
}

func unexpected(token Token, line string) error {
	return UnexpectedError{
		Line:    line,
		Invalid: token,
	}
}

func (e UnexpectedError) Error() string {
	str := e.Invalid.Literal
	if str == "" {
		str = e.Invalid.String()
	}
	return fmt.Sprintf("%s %q at %d:%d", errUnexpected, str, e.Invalid.Line, e.Invalid.Column)
}
