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

	"github.com/midbel/maestro/shell"
	"github.com/midbel/maestro/shlex"
	"golang.org/x/crypto/ssh"
)

const (
	metaDuplicate  = "DUPLICATE"
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
	metaHttpGet    = "HTTP_GET"
	metaHttpPost   = "HTTP_POST"
	metaHttpPut    = "HTTP_PUT"
	metaHttpDel    = "HTTP_DELETE"
	metaHttpPatch  = "HTTP_PATCH"
	metaHttpHead   = "HTTP_HEAD"
)

const (
	propHelp     = "help"
	propShort    = "short"
	propTags     = "tag"
	propRetry    = "retry"
	propWorkDir  = "workdir"
	propTimeout  = "timeout"
	propHosts    = "hosts"
	propError    = "error"
	propOpts     = "options"
	propArg      = "args"
	propAlias    = "alias"
	propUser     = "user"
	propGroup    = "group"
	propSchedule = "schedule"
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
	locals *Env
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
	return NewDecoderWithEnv(r, EmptyEnv())
}

func NewDecoderWithEnv(r io.Reader, env *Env) (*Decoder, error) {
	if env == nil {
		env = EmptyEnv()
	}
	d := Decoder{
		locals: env,
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
	for !d.done() {
		var err error
		switch d.curr().Type {
		case Ident:
			if d.peek().IsAssign() {
				err = d.decodeVariable(mst)
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
	switch d.curr().Literal {
	case kwInclude:
		return d.decodeInclude(mst)
	case kwExport:
		return d.decodeExport(mst)
	case kwDelete:
		return d.decodeDelete(mst)
	case kwAlias:
		return d.decodeAlias(mst)
	default:
	}
	return nil
}

func (d *Decoder) decodeInclude(mst *Maestro) error {
	type include struct {
		file     string
		optional bool
	}
	d.next()
	var list []include
	switch d.curr().Type {
	case String, Ident:
		i := include{file: d.curr().Literal}
		d.next()
		if d.curr().Type == Optional {
			i.optional = true
			d.next()
		}
		list = append(list, i)
	case BegList:
		d.next()
		if err := d.ensureEOL(); err != nil {
			return err
		}
		for !d.done() {
			if d.curr().Type == EndList {
				break
			}
			if d.curr().Type != String && d.curr().Type != Ident {
				return d.unexpected()
			}
			i := include{file: d.curr().Literal}
			d.next()
			if d.curr().Type == Optional {
				i.optional = true
				d.next()
			}
			if err := d.ensureEOL(); err != nil {
				return err
			}
			list = append(list, i)
		}
		if d.curr().Type != EndList {
			return d.unexpected()
		}
		d.next()
	default:
		return d.unexpected()
	}
	if err := d.ensureEOL(); err != nil {
		return err
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
		for !d.done() {
			if d.curr().Type == EndList {
				break
			}
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
	for !d.done() {
		if !d.curr().IsValue() {
			return d.unexpected()
		}
		d.locals.Delete(d.curr().Literal)
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeAlias(mst *Maestro) error {
	decode := func() error {
		d.setLineFunc()
		ident := d.curr()
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		if !d.curr().IsValue() {
			return d.unexpected()
		}
		d.alias[ident.Literal] = d.curr().Literal
		d.resetIdentFunc()
		d.next()
		return nil
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
		for !d.done() {
			if d.curr().Type == EndList {
				break
			}
			if err := decode(); err != nil {
				return err
			}
			if err := d.ensureEOL(); err != nil {
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

func (d *Decoder) decodeObjectVariable(mst *Maestro) error {
	// TODO
	return nil
}

func (d *Decoder) decodeVariable(mst *Maestro) error {
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

	var vs []string
	for d.curr().IsValue() && !d.done() {
		switch {
		case d.curr().IsVariable():
			xs, err := d.locals.Resolve(d.curr().Literal)
			if err != nil {
				return err
			}
			vs = append(vs, xs...)
		case d.curr().IsScript():
			xs, err := d.decodeScript(d.curr().Literal)
			if err != nil {
				return err
			}
			vs = append(vs, xs...)
		default:
			vs = append(vs, d.curr().Literal)
		}
		d.next()
	}
	if assign {
		d.locals.Define(ident.Literal, vs)
	} else {
		xs, _ := d.locals.Resolve(ident.Literal)
		d.locals.Define(ident.Literal, append(xs, vs...))
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeScript(line string) ([]string, error) {
	var (
		buf  bytes.Buffer
		opts = []shell.ShellOption{
			shell.WithEnv(d.locals),
			shell.WithStdout(&buf),
		}
		sh, _ = shell.New(opts...)
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
	cmd, err := NewSingleWithLocals(d.curr().Literal, d.locals)
	if err != nil {
		return err
	}
	for k, v := range d.env {
		cmd.shell.Export(k, v)
	}
	for k, v := range d.alias {
		cmd.shell.Alias(k, v)
	}
	cmd.Visible = !hidden
	d.next()
	if d.curr().Type == BegList {
		if err := d.decodeCommandProperties(cmd); err != nil {
			return err
		}
	}
	if d.curr().Type == Dependency {
		if err := d.decodeCommandDependencies(cmd); err != nil {
			return err
		}
	}
	if d.curr().Type == BegScript {
		if err := d.decodeCommandScripts(cmd, mst); err != nil {
			return err
		}
	}
	if err := mst.Register(cmd); err != nil {
		return err
	}
	return nil
}

func (d *Decoder) decodeCommandProperties(cmd *Single) error {
	d.next()
	d.skipNL()
	for !d.done() {
		if d.curr().Type == EndList {
			break
		}
		d.skipComment()
		switch curr := d.curr(); {
		case curr.Type == Ident:
		case curr.Type == Keyword && curr.Literal == kwAlias:
		default:
			return d.unexpected()
		}
		var (
			prop = d.curr()
			err  error
		)
		d.next()
		if d.curr().Type != Assign {
			return d.unexpected()
		}
		d.next()
		switch prop.Literal {
		default:
			err = fmt.Errorf("%s: unknown command property", prop.Literal)
		case propError:
			cmd.Error, err = d.parseString()
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
		case propUser:
			cmd.Users, err = d.parseStringList()
			sort.Strings(cmd.Users)
		case propGroup:
			cmd.Groups, err = d.parseStringList()
			sort.Strings(cmd.Groups)
		case propOpts:
			err = d.decodeCommandOptions(cmd)
		}
		if err != nil {
			return err
		}
		switch d.curr().Type {
		case Ident, String:
		case Comma:
			d.next()
			d.skipComment()
			d.skipNL()
		case Eol:
			if d.peek().Type != EndList {
				return d.unexpected()
			}
			d.next()
		case EndList:
		default:
			return d.unexpected()
		}
	}
	if d.curr().Type != EndList {
		return d.unexpected()
	}
	d.next()
	return nil
}

func (d *Decoder) decodeCommandArguments() ([]Arg, error) {
	var args []Arg
	for !d.done() && d.curr().Type != Comma {
		if d.curr().Type != Ident {
			return nil, d.unexpected()
		}
		arg := Arg{
			Name: d.curr().Literal,
		}
		d.next()
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

func (d *Decoder) decodeCommandOptions(cmd *Single) error {
	decode := func() (Option, error) {
		var opt Option
		for !d.done() {
			if d.curr().Type == EndList {
				break
			}
			d.skipComment()
			if d.curr().Type != Ident {
				return opt, d.unexpected()
			}
			var (
				prop = d.curr()
				err  error
			)
			d.next()
			if d.curr().Type != Assign {
				return opt, d.unexpected()
			}
			d.next()
			switch prop.Literal {
			default:
				return opt, fmt.Errorf("%s: unknown option property", prop.Literal)
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
			if err != nil {
				return opt, err
			}
			switch d.curr().Type {
			case Comma:
				d.next()
				d.skipNL()
			case Eol:
				if d.peek().Type != EndList {
					return opt, d.unexpected()
				}
				d.next()
			case EndList:
			default:
				return opt, d.unexpected()
			}
		}
		if d.curr().Type != EndList {
			return opt, d.unexpected()
		}
		d.next()
		return opt, nil
	}
	var done bool
	for !d.done() && !done {
		if t := d.curr().Type; t != BegList {
			if t == Ident || t == String {
				return nil
			}
			return d.unexpected()
		}
		d.next()
		d.skipNL()
		opt, err := decode()
		if err != nil {
			return err
		}
		switch d.curr().Type {
		case Comma:
			d.next()
			d.skipComment()
			d.skipNL()
			done = d.curr().Type == EndList
		case Eol:
			d.skipNL()
			done = d.curr().Type == EndList
		case EndList:
		default:
			return d.unexpected()
		}
		cmd.Options = append(cmd.Options, opt)
	}
	if !done {
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
				switch curr := d.curr(); curr.Type {
				case Ident, String, Boolean, Integer:
					args = append(args, curr.Literal)
				case Variable:
					vs, err := d.locals.Resolve(curr.Literal)
					if err != nil {
						return nil, err
					}
					args = append(args, vs...)
				default:
					return nil, d.unexpected()
				}
				d.next()
			}
			if d.curr().Type != EndList {
				return nil, d.unexpected()
			}
			d.next()
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

func (d *Decoder) decodeCommandDependencies(cmd *Single) error {
	d.next()
	for !d.done() {
		if d.curr().Type == BegScript {
			break
		}
		var optional, mandatory bool
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
		if d.curr().Type != Ident {
			return d.unexpected()
		}
		dep := Dep{
			Name:      d.curr().Literal,
			Optional:  optional,
			Mandatory: mandatory,
		}
		d.next()
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

func (d *Decoder) decodeCommandHelp(cmd *Single) error {
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

func (d *Decoder) decodeCommandScripts(cmd *Single, mst *Maestro) error {
	d.next()
	if err := d.decodeCommandHelp(cmd); err != nil {
		return err
	}
	for !d.done() {
		if d.curr().Type == EndScript {
			break
		}
		if d.curr().Type == Comment {
			d.next()
			continue
		}
		if d.curr().Type == Macro {
			if err := d.decodeScriptMacro(cmd, mst); err != nil {
				return err
			}
			continue
		}
		if d.curr().Type == Copy {
			d.next()
			other, err := mst.lookup(d.curr().Literal)
			if err != nil {
				return err
			}
			called, ok := other.(*Single)
			if !ok {
				return fmt.Errorf("call can only be made for single command")
			}
			for _, s := range called.Scripts {
				// TODO: clone/copy shell env of called called to s
				cmd.Scripts = append(cmd.Scripts, s)
			}
			d.next()
			continue
		}
		line, err := d.decodeScriptLine()
		if err != nil {
			return err
		}
		cmd.Scripts = append(cmd.Scripts, line)
	}
	if d.curr().Type != EndScript {
		return d.unexpected()
	}
	d.next()
	return d.ensureEOL()
}

func (d *Decoder) decodeScriptLine() (Line, error) {
	var (
		line Line
		seen = make(map[rune]struct{})
	)
	for d.curr().IsOperator() {
		if _, ok := seen[d.curr().Type]; ok {
			return line, fmt.Errorf("operator already set")
		}
		seen[d.curr().Type] = struct{}{}
		switch d.curr().Type {
		case Echo:
			line.Echo = !line.Echo
		case Reverse:
			line.Reverse = !line.Reverse
		case Ignore:
			line.Ignore = !line.Ignore
		case Subshell:
			line.Subshell = !line.Subshell
		default:
			return line, d.unexpected()
		}
		d.next()
	}
	if d.curr().Type != Script {
		return line, d.unexpected()
	}
	line.Line = d.curr().Literal
	d.next()

	return line, nil
}

func (d *Decoder) decodeScriptMacro(cmd *Single, mst *Maestro) error {
	var err error
	switch macro := d.curr().Literal; macro {
	case macroRepeat:
		err = decodeMacroRepeat(d, cmd)
	case macroSequence:
		err = decodeMacroSequence(d, cmd)
	default:
		err = fmt.Errorf("%s: unknown macro", macro)
	}
	return err
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
	case metaDuplicate:
		mst.Duplicate, err = d.parseString()
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

func (d *Decoder) ensureEOL() error {
	switch d.curr().Type {
	case Eol, Comment:
		d.next()
	default:
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) parseStringList() ([]string, error) {
	if d.curr().Type == Eol || d.curr().Type == Comment {
		return nil, nil
	}
	var str []string
	for d.curr().IsValue() {
		if d.curr().IsVariable() {
			vs, err := d.locals.Resolve(d.curr().Literal)
			if err != nil {
				return nil, err
			}
			str = append(str, vs...)
		} else {
			str = append(str, d.curr().Literal)
		}
		d.next()
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
	defer d.next()

	str := d.curr().Literal
	if d.curr().IsVariable() {
		vs, err := d.locals.Resolve(d.curr().Literal)
		if err != nil {
			return "", err
		}
		if len(vs) >= 0 {
			str = vs[0]
		}
	}
	return str, nil
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

func (d *Decoder) skipNL() {
	for d.curr().Type == Eol {
		d.next()
	}
}

func (d *Decoder) skipComment() {
	for d.curr().Type == Comment {
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
	var (
		curr = d.curr()
		str  = curr.Literal
	)
	if str == "" {
		str = curr.String()
	}
	return fmt.Errorf("maestro: %w %s at %d:%d", errUnexpected, str, curr.Line, curr.Column)
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
	d.locals = EnclosedEnv(d.locals)
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

func (d *Decoder) setLineFunc() {
	z := len(d.frames)
	if z == 0 {
		return
	}
	d.frames[z-1].scan.SetIdentFunc(IsLine)
}

func (d *Decoder) setValueFunc() {
	z := len(d.frames)
	if z == 0 {
		return
	}
	d.frames[z-1].scan.SetIdentFunc(IsValue)
}

func (d *Decoder) resetIdentFunc() {
	z := len(d.frames)
	if z == 0 {
		return
	}
	d.frames[z-1].scan.ResetIdentFunc()
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

func (f *frame) next() {
	f.curr = f.peek
	f.peek = f.scan.Scan()
}

func (f *frame) done() bool {
	return f.curr.IsEOF()
}
