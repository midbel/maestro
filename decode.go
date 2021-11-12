package maestro

import (
	"errors"
	"fmt"
	"io"
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
	propUsage   = "usage"
	propTags    = "tag"
	propRetry   = "retry"
	propWorkDir = "workdir"
	propTimeout = "timeout"
	propHosts   = "host"
	propError   = "error"
	propArgs    = "args"
)

type Decoder struct {
	scan *Scanner
	curr Token
	peek Token

	env map[string][]string
}

func Decode(r io.Reader) (*Maestro, error) {
	s, err := Scan(r)
	if err != nil {
		return nil, err
	}
	d := Decoder{
		scan: s,
		env:  make(map[string][]string),
	}
	d.next()
	d.next()
	return d.Decode()
}

func (d *Decoder) Decode() (*Maestro, error) {
	mst := Maestro{
		Commands: make(map[string]Command),
	}
	for !d.done() {
		var err error
		switch d.curr.Type {
		case Ident:
			if d.peek.Type == Assign {
				err = d.decodeVariable(&mst)
				break
			}
			err = d.decodeCommand(&mst)
		case Meta:
			err = d.decodeMeta(&mst)
		case Comment:
			d.next()
		default:
			err = d.unexpected()
		}
		if err != nil {
			return nil, err
		}
	}
	return &mst, nil
}

func (d *Decoder) decodeVariable(mst *Maestro) error {
	ident := d.curr
	d.next()
	if d.curr.Type != Assign {
		return d.unexpected()
	}
	d.next()
	for d.curr.IsValue() && !d.done() {
		d.env[ident.Literal] = append(d.env[ident.Literal], d.curr.Literal)
		d.next()
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeCommand(mst *Maestro) error {
	cmd := NewSingleWithLocals(d.curr.Literal, d.env)
	d.next()
	if d.curr.Type == BegList {
		if err := d.decodeCommandProperties(cmd); err != nil {
			return err
		}
	}
	if d.curr.Type == Dependency {
		if err := d.decodeCommandDependencies(cmd); err != nil {
			return err
		}
	}
	if d.curr.Type == BegScript {
		if err := d.decodeCommandScripts(cmd); err != nil {
			return err
		}
	}
	if err := mst.Register(cmd); err != nil {
		return err
	}
	return d.ensureEOL()
}

func (d *Decoder) decodeCommandProperties(cmd *Single) error {
	d.next()
	for !d.done() {
		if d.curr.Type == EndList {
			break
		}
		if d.curr.Type != Ident {
			return d.unexpected()
		}
		var (
			prop = d.curr
			err  error
		)
		d.next()
		if d.curr.Type != Assign {
			return d.unexpected()
		}
		d.next()
		switch prop.Literal {
		default:
			err = fmt.Errorf("%s: unknown command property", prop.Literal)
		case propError:
			cmd.Error, err = d.parseString()
		case propHelp:
			cmd.Help, err = d.parseString()
		case propUsage:
			cmd.Usage, err = d.parseString()
		case propTags:
			cmd.Tags, err = d.parseStringList()
		case propRetry:
			cmd.Retry, err = d.parseInt()
		case propTimeout:
			cmd.Timeout, err = d.parseDuration()
		case propHosts:
			cmd.Tags, err = d.parseStringList()
		case propArgs:
		}
		if err != nil {
			return err
		}

		switch d.curr.Type {
		case Comma:
			d.next()
		case EndList:
		default:
			return d.unexpected()
		}
	}
	if d.curr.Type != EndList {
		return d.unexpected()
	}
	d.next()
	return nil
}

func (d *Decoder) decodeCommandDependencies(cmd *Single) error {
	d.next()
	for !d.done() {
		if d.curr.Type == BegScript {
			break
		}
		if d.curr.Type != Ident {
			return d.unexpected()
		}
		dep := Dep{
			Name: d.curr.Literal,
		}
		cmd.Dependencies = append(cmd.Dependencies, dep)
		d.next()
		if d.curr.Type == Background {
			d.next()
			dep.Bg = true
		}
		switch d.curr.Type {
		case Comma:
			d.next()
		case BegScript:
		default:
			return d.unexpected()
		}
	}
	if d.curr.Type != BegScript {
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) decodeCommandScripts(cmd *Single) error {
	d.next()
	for !d.done() {
		if d.curr.Type == EndScript {
			break
		}
		if d.curr.Type != Script {
			return d.unexpected()
		}
		cmd.Scripts = append(cmd.Scripts, d.curr.Literal)
		d.next()
	}
	if d.curr.Type != EndScript {
		return d.unexpected()
	}
	d.next()
	return nil
}

func (d *Decoder) decodeMeta(mst *Maestro) error {
	var (
		meta = d.curr
		err  error
	)
	d.next()
	if d.curr.Type != Assign {
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
	switch d.curr.Type {
	case Eol, Comment:
		d.next()
	default:
		return d.unexpected()
	}
	return nil
}

func (d *Decoder) parseStringList() ([]string, error) {
	if d.curr.Type == Eol || d.curr.Type == Comment {
		return nil, nil
	}
	var str []string
	for d.curr.IsValue() {
		if d.curr.IsVariable() {
			vs, ok := d.env[d.curr.Literal]
			if !ok {
				return nil, d.undefined()
			}
			str = append(str, vs...)
		} else {
			str = append(str, d.curr.Literal)
		}
		d.next()
	}
	return str, nil
}

func (d *Decoder) parseString() (string, error) {
	if d.curr.Type == Eol || d.curr.Type == Comment {
		return "", nil
	}
	if !d.curr.IsValue() {
		return "", d.unexpected()
	}
	defer d.next()

	str := d.curr.Literal
	if d.curr.IsVariable() {
		vs, ok := d.env[d.curr.Literal]
		if !ok {
			return "", d.undefined()
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

func (d *Decoder) next() {
	d.curr = d.peek
	d.peek = d.scan.Scan()
}

func (d *Decoder) done() bool {
	return d.curr.IsEOF()
}

func (d *Decoder) unexpected() error {
	return fmt.Errorf("%w %s", errUnexpected, d.curr)
}

func (d *Decoder) undefined() error {
	return fmt.Errorf("%s: %w", d.curr.Literal, errUndefined)
}

var (
	errUnexpected = errors.New("unexpected token")
	errUndefined  = errors.New("undefined variable")
)
