package maestro

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/maestro/internal/env"
	"github.com/midbel/maestro/internal/scan"
	"github.com/midbel/maestro/internal/validate"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
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
	propHelp    = "help"
	propShort   = "short"
	propTags    = "tag"
	propRetry   = "retry"
	propWorkDir = "workdir"
	propTimeout = "timeout"
	propHosts   = "hosts"
	propOpts    = "options"
	propArgs    = "args"
	propAlias   = "alias"
)

const (
	optShort   = "short"
	optLong    = "long"
	optDefault = "default"
	optFlag    = "flag"
	optHelp    = "help"
	optValid   = "check"
)

const (
	sshAddr  = "addr"
	sshUser  = "user"
	sshPass  = "pass"
	sshKey   = "key"
	sshHosts = "known_hosts"
)

type Decoder struct {
	dir string

	env   *env.Env // environment variables
	vars  *env.Env // locals variables
	alias *env.Env // alias definitions

	scan *scan.Scanner
	curr scan.Token
	peek scan.Token
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	return NewDecoderWithEnv(r, env.Empty())
}

func NewDecoderWithEnv(r io.Reader, vars *env.Env) (*Decoder, error) {
	scan, err := scan.Scan(r)
	if err != nil {
		return nil, err
	}
	dec := &Decoder{
		scan:  scan,
		vars:  vars,
		env:   env.Empty(),
		alias: env.Empty(),
	}
	if n, ok := r.(interface{ Name() string }); ok {
		dec.dir = filepath.Dir(n.Name())
	}
	dec.next()
	dec.next()
	return dec, nil
}

func (d *Decoder) Decode(mst *Maestro) error {
	var err error
	for !d.done() {
		d.skipEol()
		switch d.curr.Type {
		case scan.Meta:
			err = d.decodeMeta(mst)
		case scan.Ident:
			err = d.decodeIdent(mst)
		case scan.Hidden:
			err = d.decodeCommand(mst)
		case scan.Keyword:
			err = d.decodeKeyword(mst)
		default:
			return d.unexpected("decode")
		}
		if err != nil {
			break
		}
	}
	return err
}

func (d *Decoder) decodeKeyword(mst *Maestro) error {
	var err error
	switch d.curr.Literal {
	case scan.KwAlias:
		err = d.decodeAlias(mst)
	case scan.KwInclude:
		err = d.decodeInclude(mst)
	case scan.KwExport:
		err = d.decodeExport(mst)
	case scan.KwDelete:
		err = d.decodeDelete(mst)
	default:
		err = fmt.Errorf("%s unknown keyword", d.curr.Literal)
	}
	return err
}

func (d *Decoder) decodeIdent(mst *Maestro) error {
	if d.peek.Type == scan.Assign || d.peek.Type == scan.Append {
		return d.decodeAssign(mst)
	}
	return d.decodeCommand(mst)
}

func (d *Decoder) decodeMeta(mst *Maestro) error {
	var (
		ident = d.curr.Literal
		err   error
	)
	d.next()
	if err = d.expect(scan.Assign, "expected = after .%s", ident); err != nil {
		return err
	}
	d.next()
	switch ident {
	case metaWorkDir:
		mst.MetaExec.WorkDir, err = d.decodeDirectory()
	case metaTrace:
		mst.MetaExec.Trace, err = d.decodeBool()
	case metaAll:
		mst.MetaExec.All, err = d.decodeStringList()
	case metaDefault:
		mst.MetaExec.Default, err = d.decodeString()
	case metaBefore:
		mst.MetaExec.Before, err = d.decodeStringList()
	case metaAfter:
		mst.MetaExec.After, err = d.decodeStringList()
	case metaError:
		mst.MetaExec.Error, err = d.decodeStringList()
	case metaSuccess:
		mst.MetaExec.Success, err = d.decodeStringList()
	case metaAuthor:
		mst.MetaAbout.Author, err = d.decodeString()
	case metaEmail:
		mst.MetaAbout.Email, err = d.decodeString()
	case metaVersion:
		mst.MetaAbout.Version, err = d.decodeString()
	case metaUsage:
		mst.MetaAbout.Usage, err = d.decodeString()
	case metaHelp:
		mst.MetaAbout.Help, err = d.decodeString()
	case metaUser:
		mst.MetaSSH.User, err = d.decodeString()
	case metaPass:
		mst.MetaSSH.Pass, err = d.decodeString()
	case metaPubKey:
		mst.MetaSSH.Key, err = d.decodeSigner()
	case metaKnownHosts:
		mst.MetaSSH.KnownHosts, err = d.decodeKnownHosts()
	case metaParallel:
		mst.MetaSSH.Parallel, err = d.decodeInt()
	case metaCertFile:
		mst.MetaHttp.CertFile, err = d.decodeString()
	case metaKeyFile:
		mst.MetaHttp.KeyFile, err = d.decodeString()
	default:
		err = fmt.Errorf(".%s meta not recognized", ident)
	}
	if err == nil {
		err = d.ensureEol()
	}
	return err
}

func (d *Decoder) decodeAssign(mst *Maestro) error {
	var (
		ident  = d.curr.Literal
		assign bool
	)
	d.next()
	switch d.curr.Type {
	case scan.Assign:
		assign = true
	case scan.Append:
	default:
		return d.unexpected("decoding assignment")
	}
	d.next()
	values, err := d.decodeStringList()
	if err != nil {
		return err
	}
	if assign {
		d.vars.Define(ident, values)
	} else {
		err = d.vars.Append(ident, values)
	}
	return err
}

func (d *Decoder) decodeCommand(mst *Maestro) error {
	var hidden bool
	if hidden = d.is(scan.Hidden); hidden {
		d.next()
	}
	cmd := CommandSettings{
		Name:    d.curr.Literal,
		Visible: !hidden,
		vars:    d.vars.Copy(),
		env:     d.env.Copy(),
		alias:   d.env.Copy(),
	}
	d.next()
	if d.is(scan.BegList) {
		if err = d.decodeCommandProperties(&cmd); err != nil {
			return err
		}
	}
	if d.is(scan.Dependency) {
		if err = d.decodeCommandDependencies(&cmd); err != nil {
			return err
		}
	}
	if d.is(scan.BegScript) {
		if err = d.decodeCommandScript(&cmd); err != nil {
			return err
		}
	}
	return mst.Register(cmd)
}

func (d *Decoder) decodeCommandProperties(cmd *CommandSettings) error {
	return d.decodeProperties(func(ident string) error {
		var err error
		switch ident {
		case propHelp:
			cmd.Desc, err = d.decodeString()
		case propShort:
			cmd.Short, err = d.decodeString()
		case propTags:
			cmd.Categories, err = d.decodeStringList()
		case propRetry:
			cmd.Retry, err = d.decodeInt()
		case propWorkDir:
			cmd.WorkDir, err = d.decodeDirectory()
		case propTimeout:
			cmd.Timeout, err = d.decodeDuration()
		case propAlias:
			cmd.Alias, err = d.decodeStringList()
			sort.Strings(cmd.Alias)
		case propHosts:
			err = d.decodeCommandHosts(cmd)
		case propOpts:
			err = d.decodeCommandOptions(cmd)
		case propArgs:
			err = d.decodeCommandArguments(cmd)
		default:
			err = fmt.Errorf("%s command property not recognized", ident)
		}
		return err
	})
}

func (d *Decoder) decodeCommandDependencies(cmd *CommandSettings) error {
	d.next()
	for !d.done() && !d.is(scan.BegScript) && !d.is(scan.Eol) {
		if err := d.expect(scan.Ident, "identifier expected"); err != nil {
			return err
		}
		cmd.Deps = append(cmd.Deps, createDep(d.curr.Literal))
		d.next()
		switch d.curr.Type {
		case scan.Comma:
			d.next()
			if d.is(scan.BegScript) {
				return d.unexpected("decoding command dependencies")
			}
		case scan.BegScript, scan.Eol, scan.Eof:
		default:
			return d.unexpected("decoding command dependencies")
		}
	}
	return nil
}

func (d *Decoder) decodeCommandScript(cmd *CommandSettings) error {
	var body func() ([]CommandScript, error)
	body = func() ([]CommandScript, error) {
		var lines []CommandScript
		for !d.done() && !d.is(scan.EndScript) {
			d.skipEol()
			switch d.curr.Type {
			case scan.Script:
				lines = append(lines, createScript(d.curr.Literal))
				d.next()
			case scan.BegScript:
				d.next()
				ls, err := body()
				if err != nil {
					return nil, err
				}
				lines[len(lines)-1].Body = ls
			default:
				return nil, d.unexpected("decoding script body")
			}
		}
		if err := d.expect(scan.EndScript, "expected } at end of script"); err != nil {
			return nil, err
		}
		d.next()
		return lines, nil
	}
	d.next()
	if err := d.decodeCommandHelp(cmd); err != nil {
		return err
	}
	lines, err := body()
	if err == nil {
		cmd.Lines = lines
	}
	return err
}

func (d *Decoder) decodeCommandHelp(cmd *CommandSettings) error {
	d.skipEol()
	var (
		help strings.Builder
		prev string
	)
	for !d.done() && d.is(scan.Comment) {
		str := d.curr.Literal
		if str == "" && prev == "" {
			d.next()
			continue
		}
		help.WriteString(strings.TrimSpace(str))
		help.WriteString("\n")
		prev = str
		d.next()
	}
	if help.Len() > 0 {
		cmd.Desc = strings.TrimSpace(help.String())
	}
	return nil
}

func (d *Decoder) decodeCommandHosts(cmd *CommandSettings) error {
	if !d.is(scan.BegList) {
		list, err := d.decodeStringList()
		if err != nil {
			return err
		}
		for i := range list {
			cmd.Hosts = append(cmd.Hosts, createTarget(list[i]))
		}
		return err
	}
	var target, zero CommandTarget
	get := func(ident string) error {
		var err error
		switch ident {
		case sshAddr:
			target.Addr, err = d.decodeString()
		case sshUser:
			target.User, err = d.decodeString()
		case sshPass:
			target.Pass, err = d.decodeString()
		case sshKey:
			target.Key, err = d.decodeSigner()
		case sshHosts:
			target.KnownHosts, err = d.decodeKnownHosts()
		default:
			err = fmt.Errorf("%s host property not recognized", ident)
		}
		return err
	}
	for !d.done() && d.is(scan.BegList) {
		err := d.decodeObject(get)
		if err != nil {
			return err
		}
		cmd.Hosts = append(cmd.Hosts, target)
		target = zero
		switch d.curr.Type {
		case scan.Comma:
			if d.peek.Type == scan.BegList {
				d.next()
			}
		default:
			return nil
		}
	}
	return nil
}

func (d *Decoder) decodeCommandOptions(cmd *CommandSettings) error {
	var option, zero CommandOption
	get := func(ident string) error {
		var err error
		switch ident {
		case optShort:
			option.Short, err = d.decodeString()
		case optLong:
			option.Long, err = d.decodeString()
		case optDefault:
			option.Default, err = d.decodeString()
		case optFlag:
			option.Flag, err = d.decodeBool()
		case optHelp:
			option.Help, err = d.decodeString()
		case optValid:
			option.Valid, err = d.decodeValidFunc()
		default:
			err = fmt.Errorf("%s option property not recognized", ident)
		}
		return err
	}
	for !d.done() && d.is(scan.BegList) {
		err := d.decodeObject(get)
		if err != nil {
			return err
		}
		cmd.Options = append(cmd.Options, option)
		option = zero
		switch d.curr.Type {
		case scan.Comma:
			if d.peek.Type == scan.BegList {
				d.next()
			}
		default:
			return nil
		}
	}
	return nil
}

func (d *Decoder) decodeCommandArguments(cmd *CommandSettings) error {
	for !d.done() && !d.is(scan.Comma) && !d.is(scan.Eol) && !d.is(scan.EndList) {
		if err := d.expect(scan.Ident, "identifier expected"); err != nil {
			return err
		}
		arg := createArg(d.curr.Literal)
		d.next()
		if d.is(scan.BegList) {
			a, err := d.decodeValidFunc()
			if err != nil {
				return err
			}
			arg.Valid = a
		}
		cmd.Args = append(cmd.Args, arg)
	}
	return nil
}

func (d *Decoder) decodeObject(do func(string) error) error {
	accept := func(tok scan.Token) error {
		if tok.Type == scan.Ident {
			return nil
		}
		return d.unexpected("decoding list")
	}
	return d.decodeList(accept, do)
}

func (d *Decoder) decodeProperties(do func(ident string) error) error {
	accept := func(tok scan.Token) error {
		if tok.Type == scan.Ident || (tok.Type == scan.Keyword && tok.Literal == scan.KwAlias) {
			return nil
		}
		return d.unexpected("decoding properties")
	}
	return d.decodeList(accept, do)
}

type (
	acceptFunc func(scan.Token) error
	identFunc  func(string) error
)

func (d *Decoder) decodeList(accept acceptFunc, do identFunc) error {
	if err := d.expect(scan.BegList, "expected ( at begin of list"); err != nil {
		return err
	}
	d.next()
	for !d.done() && !d.is(scan.EndList) {
		d.skipEol()
		if err := accept(d.curr); err != nil {
			return err
		}
		ident := d.curr.Literal
		d.next()
		if err := d.expect(scan.Assign, "expected = after %s", ident); err != nil {
			return err
		}
		d.next()
		if err := do(ident); err != nil {
			return err
		}
		switch d.curr.Type {
		case scan.Comma:
			d.next()
		case scan.Eol, scan.Comment:
			d.skipEol()
			if err := d.expect(scan.EndList, "expect ) after eol/comment"); err != nil {
				return err
			}
		case scan.EndList:
		default:
			return d.unexpected("decoding object")
		}
	}
	if err := d.expect(scan.EndList, "expected ) at end of list"); err != nil {
		return err
	}
	d.next()
	return nil
}

func (d *Decoder) decodeAlias(mst *Maestro) error {
	get := func() error {
		if err := d.expect(scan.Ident, "identifier expected"); err != nil {
			return err
		}
		ident := d.curr.Literal
		d.next()
		if err := d.expect(scan.Assign, "expected = after %s", ident); err != nil {
			return err
		}
		d.next()
		alias, err := d.decodeStringList()
		if err == nil {
			d.alias.Define(ident, alias)
		}
		return d.ensureEol()
	}
	return d.decodeKw(get)
}

func (d *Decoder) decodeInclude(mst *Maestro) error {
	get := func() error {
		file, err := d.decodeString()
		if err != nil {
			return err
		}
		r, err := os.Open(file)
		if err != nil {
			r, err = os.Open(filepath.Join(d.dir, file))
			if err != nil {
				return err
			}
		}
		defer r.Close()

		dec, err := NewDecoderWithEnv(r, env.Enclosed(d.vars))
		if err != nil {
			return err
		}
		if err := dec.Decode(mst); err != nil {
			return err
		}
		return d.ensureEol()
	}
	return d.decodeKw(get)
}

func (d *Decoder) decodeExport(mst *Maestro) error {
	get := func() error {
		if err := d.expect(scan.Ident, "identifer expected"); err != nil {
			return err
		}
		ident := d.curr.Literal
		d.next()
		if err := d.expect(scan.Assign, "expected = after %s", ident); err != nil {
			return err
		}
		d.next()
		values, err := d.decodeStringList()
		if err != nil {
			return err
		}
		d.env.Define(ident, values)
		return d.ensureEol()
	}
	return d.decodeKw(get)
}

func (d *Decoder) decodeDelete(mst *Maestro) error {
	d.next()
	for !d.is(scan.Eol) && !d.done() {
		if err := d.expect(scan.Ident, "identifier expected"); err != nil {
			return err
		}
		d.vars.Delete(d.curr.Literal)
		d.next()
	}
	return d.ensureEol()
}

func (d *Decoder) decodeKw(do func() error) error {
	d.next()
	if !d.is(scan.BegList) {
		return do()
	}
	d.next()
	d.skipEol()
	for !d.is(scan.EndList) && !d.done() {
		if err := do(); err != nil {
			return err
		}
	}
	if err := d.expect(scan.EndList, "expected ) at end of list"); err != nil {
		return err
	}
	d.next()
	return d.ensureEol()
}

func (d *Decoder) decodeValidFunc() (validate.ValidateFunc, error) {
	var (
		decodeArgs func() ([]string, error)
		decodeFunc func(func() bool) ([]validate.ValidateFunc, error)
	)

	decodeArgs = func() ([]string, error) {
		if !d.is(scan.BegList) {
			return nil, nil
		}
		d.next()
		var args []string
		for !d.done() && !d.is(scan.EndList) {
			str, err := d.decodeString()
			if err != nil {
				return nil, err
			}
			args = append(args, str)
		}
		if err := d.expect(scan.EndList, "expected ) after arguments list"); err != nil {
			return nil, err
		}
		d.next()
		return args, nil
	}

	decodeFunc = func(accept func() bool) ([]validate.ValidateFunc, error) {
		var all []validate.ValidateFunc
		for !d.done() && !accept() {
			if err := d.expect(scan.Ident, "identifier expected"); err != nil {
				return nil, err
			}
			ident := d.curr.Literal
			d.next()
			switch ident {
			case validate.ValidAll, validate.ValidSome, validate.ValidNot:
				if err := d.expect(scan.BegList, "expected ("); err != nil {
					return nil, err
				}
				d.next()
				set, err := decodeFunc(func() bool {
					return d.is(scan.EndList)
				})
				if err != nil {
					return nil, err
				}
				valid, err := validate.Get(ident, set...)
				if err != nil {
					return nil, err
				}
				all = append(all, valid)
			default:
				args, err := decodeArgs()
				if err != nil {
					return nil, err
				}
				valid, err := validate.GetValidateFunc(ident, args)
				if err != nil {
					return nil, err
				}
				all = append(all, valid)
			}
		}
		if !accept() {
			return nil, d.unexpected("decoding validation rules")
		}
		d.next()
		return all, nil
	}

	until := func() bool {
		return d.is(scan.Eol) || d.is(scan.Comma) || d.is(scan.EndList)
	}
	if d.is(scan.BegList) {
		d.next()
		until = func() bool { return d.is(scan.EndList) }
	}

	set, err := decodeFunc(until)
	if err != nil {
		return nil, err
	}
	return validate.All(set...), nil
}

func (d *Decoder) decodeVariable() ([]string, error) {
	return d.vars.Resolve(d.curr.Literal)
}

func (d *Decoder) decodeKnownHosts() (ssh.HostKeyCallback, error) {
	files, err := d.decodeStringList()
	if err != nil {
		return nil, err
	}
	return knownhosts.New(files...)
}

func (d *Decoder) decodeSigner() (ssh.Signer, error) {
	file, err := d.decodeString()
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(buf)
}

func (d *Decoder) decodeInt() (int64, error) {
	str, err := d.decodeString()
	if err == nil {
		return strconv.ParseInt(str, 0, 64)
	}
	return 0, err
}

func (d *Decoder) decodeBool() (bool, error) {
	str, err := d.decodeString()
	if err == nil {
		return strconv.ParseBool(str)
	}
	return false, err
}

func (d *Decoder) decodeDuration() (time.Duration, error) {
	str, err := d.decodeString()
	if err == nil {
		return time.ParseDuration(str)
	}
	return 0, err
}

func (d *Decoder) decodeDirectory() (string, error) {
	str, err := d.decodeString()
	if err != nil {
		return "", err
	}
	i, err := os.Stat(str)
	if err != nil {
		return "", err
	}
	if !i.IsDir() {
		return "", fmt.Errorf("%s not a directory", str)
	}
	return str, nil
}

func (d *Decoder) decodeString() (string, error) {
	defer d.next()
	var str string
	switch d.curr.Type {
	case scan.Ident, scan.String, scan.Boolean:
		str = d.curr.Literal
	case scan.Quote:
		return d.decodeQuote()
	case scan.Variable:
		s, err := d.decodeVariable()
		if err != nil {
			return "", err
		}
		str = strings.Join(s, "")
	default:
		return "", d.unexpected("decoding string")
	}
	return str, nil
}

func (d *Decoder) decodeQuote() (string, error) {
	d.next()
	var str []string
	for !d.is(scan.Quote) && !d.done() {
		switch d.curr.Type {
		case scan.String:
			str = append(str, d.curr.Literal)
		case scan.Variable:
			s, err := d.decodeVariable()
			if err != nil {
				return "", err
			}
			str = append(str, s...)
		default:
			return "", d.unexpected("decoding quoted string")
		}
		d.next()
	}
	return strings.Join(str, ""), nil
}

func (d *Decoder) decodeStringList() ([]string, error) {
	var str []string
	for !d.done() {
		switch d.curr.Type {
		case scan.String, scan.Boolean, scan.Ident:
			str = append(str, d.curr.Literal)
		case scan.Quote:
			s, err := d.decodeQuote()
			if err != nil {
				return nil, err
			}
			str = append(str, s)
		case scan.Variable:
			s, err := d.decodeVariable()
			if err != nil {
				return nil, err
			}
			str = append(str, s...)
		default:
			return str, nil
		}
		d.next()
	}
	return str, nil
}

func (d *Decoder) skipEol() {
	for d.is(scan.Eol) || d.is(scan.Comment) {
		d.next()
	}
}

func (d *Decoder) ensureEol() error {
	switch d.curr.Type {
	case scan.Eol, scan.Eof, scan.Comment:
		d.next()
	default:
		return d.unexpected("at end of line")
	}
	return nil
}

func (d *Decoder) done() bool {
	return d.is(scan.Eof)
}

func (d *Decoder) expect(kind rune, msg string, args ...interface{}) error {
	if d.is(kind) {
		return nil
	}
	return fmt.Errorf(msg, args...)
}

func (d *Decoder) is(kind rune) bool {
	return d.curr.Type == kind
}

func (d *Decoder) next() {
	d.curr = d.peek
	d.peek = d.scan.Scan()
}

func (d *Decoder) unexpected(where string) error {
	return fmt.Errorf("%s: %s unexpected at %s", where, d.curr, d.curr.Position)
}
