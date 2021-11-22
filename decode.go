package maestro

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

const (
	metaDuplicate = "DUPLICATE"
	metaWorkDir   = "WORKDIR"
	metaPath      = "PATH"
	metaEcho      = "ECHO"
	metaParallel  = "PARALLEL"
	metaAll       = "ALL"
	metaDefault   = "DEFAULT"
	metaBefore    = "BEFORE"
	metaAfter     = "AFTER"
	metaError     = "ERROR"
	metaSuccess   = "SUCCESS"
	metaAuthor    = "AUTHOR"
	metaEmail     = "EMAIL"
	metaVersion   = "VERSION"
	metaUsage     = "USAGE"
	metaHelp      = "HELP"
	metaUser      = "USER"
	metaPass      = "PASSWORD"
	metaPrivKey   = "PRIVATEKEY"
	metaPubKey    = "PUBLICKEY"
)

const (
	propHelp    = "help"
	propShort   = "short"
	propUsage   = "usage"
	propTags    = "tag"
	propRetry   = "retry"
	propWorkDir = "workdir"
	propTimeout = "timeout"
	propHosts   = "host"
	propError   = "error"
	propArgs    = "options"
)

const (
	optShort    = "short"
	optLong     = "long"
	optRequired = "required"
	optDefault  = "default"
	optFlag     = "flag"
	optHelp     = "help"
)

type Decoder struct {
	locals *Env
	env    map[string]string
	frames []*frame
}

func Decode(r io.Reader) (*Maestro, error) {
	d := Decoder{
		locals: EmptyEnv(),
		env:    make(map[string]string),
	}
	if err := d.push(r); err != nil {
		return nil, err
	}
	return d.Decode()
}

func (d *Decoder) Decode() (*Maestro, error) {
	mst := New()
	for !d.done() {
		var err error
		switch d.curr().Type {
		case Ident:
			if d.peek().Type == Assign {
				err = d.decodeVariable(mst)
				break
			}
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
			return nil, err
		}
	}
	return mst, nil
}

func (d *Decoder) decodeKeyword(mst *Maestro) error {
	switch d.curr().Literal {
	case kwInclude:
		return d.decodeInclude()
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

func (d *Decoder) decodeInclude() error {
	d.next()
	var list []string
	switch d.curr().Type {
	case String, Ident:
		list = append(list, d.curr().Literal)
		d.next()
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
			list = append(list, d.curr().Literal)
			d.next()
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
	if err := d.ensureEOL(); err != nil {
		return err
	}
	for i := range list {
		if err := d.decodeFile(list[i]); err != nil {
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
		mst.Alias[ident.Literal] = d.curr().Literal
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

func (d *Decoder) decodeVariable(mst *Maestro) error {
	ident := d.curr()
	d.next()
	if d.curr().Type != Assign {
		return d.unexpected()
	}
	d.next()

	var vs []string
	for d.curr().IsValue() && !d.done() {
		if d.curr().IsVariable() {
			xs, err := d.locals.Resolve(d.curr().Literal)
			if err != nil {
				return err
			}
			vs = append(vs, xs...)
		} else {
			vs = append(vs, d.curr().Literal)
		}
		d.next()
	}
	d.locals.Define(ident.Literal, vs)
	return d.ensureEOL()
}

func (d *Decoder) decodeCommand(mst *Maestro) error {
	cmd, err := NewSingleWithLocals(d.curr().Literal, d.locals)
	if err != nil {
		return err
	}
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
		if d.curr().Type != Ident {
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
		case propUsage:
			cmd.Usage, err = d.parseString()
		case propTags:
			cmd.Cats, err = d.parseStringList()
		case propRetry:
			cmd.Retry, err = d.parseInt()
		case propTimeout:
			cmd.Timeout, err = d.parseDuration()
		case propHosts:
			cmd.Hosts, err = d.parseStringList()
		case propArgs:
			err = d.decodeCommandOptions(cmd)
		}
		if err != nil {
			return err
		}
		switch d.curr().Type {
		case Comma:
			d.next()
			d.skipComment()
			d.skipNL()
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
			}
			if err != nil {
				return opt, err
			}
			switch d.curr().Type {
			case Comma:
				d.next()
				d.skipNL()
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
		if d.curr().Type != BegList {
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

func (d *Decoder) decodeCommandDependencies(cmd *Single) error {
	d.next()
	for !d.done() {
		if d.curr().Type == BegScript {
			break
		}
		if d.curr().Type != Ident {
			return d.unexpected()
		}
		dep := Dep{
			Name: d.curr().Literal,
		}
		cmd.Dependencies = append(cmd.Dependencies, dep)
		d.next()
		if d.curr().Type == Background {
			d.next()
			dep.Bg = true
		}
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

func (d *Decoder) decodeCommandScripts(cmd *Single, mst *Maestro) error {
	d.next()
Script:
	for !d.done() {
		if d.curr().Type == EndScript {
			break
		}
		if d.curr().Type == Comment {
			d.next()
			continue
		}
		var (
			i    Line
			seen = make(map[rune]struct{})
		)
		for d.curr().IsOperator() {
			if _, ok := seen[d.curr().Type]; ok {
				return fmt.Errorf("operator already set")
			}
			seen[d.curr().Type] = struct{}{}
			switch d.curr().Type {
			case Echo:
				i.Echo = !i.Echo
			case Reverse:
				i.Reverse = !i.Reverse
			case Ignore:
				i.Ignore = !i.Ignore
			case Isolated:
				i.Empty = !i.Empty
			case Call:
				// TODO: revise own Call is handled + give option to pass arguments to call
				// NOTE: when Call is encountered, other line operator are discarded!
				d.next()
				other, err := mst.lookup(d.curr().Literal)
				if err != nil {
					return err
				}
				single, ok := other.(*Single)
				if !ok {
					return fmt.Errorf("call can only be made for single command")
				}
				cmd.Scripts = append(cmd.Scripts, single.Scripts...)
				d.next()
				continue Script
			default:
				return d.unexpected()
			}
			d.next()
		}
		if d.curr().Type != Script {
			return d.unexpected()
		}
		i.Line = d.curr().Literal
		cmd.Scripts = append(cmd.Scripts, i)
		d.next()
	}
	if d.curr().Type != EndScript {
		return d.unexpected()
	}
	d.next()
	return d.ensureEOL()
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
	case metaPath:
		mst.MetaExec.Path, err = d.parseStringList()
	case metaEcho:
		mst.MetaExec.Echo, err = d.parseBool()
	case metaParallel:
		mst.MetaExec.Parallel, err = d.parseInt()
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
	case metaPrivKey:
	case metaPubKey:
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
	curr := d.curr()
	return fmt.Errorf("%w %s at %d:%d", errUnexpected, curr.Literal, curr.Line, curr.Column)
}

func (d *Decoder) undefined() error {
	return fmt.Errorf("%s: %w", d.curr().Literal, errUndefined)
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
