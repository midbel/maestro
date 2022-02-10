package maestro

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	macroRepeat   = "repeat"
	macroSequence = "sequence"
	macroVar      = "<var>"
	macroIter     = "<iter>"
	macroIter0    = "<iter0>"
)

func decodeMacroSequence(d *Decoder, cmd *Single) error {
	d.next()
	if d.curr().Type != BegScript {
		return d.unexpected()
	}
	d.next()

	var lines []Line
	for !d.done() && d.curr().Type != EndScript {
		if d.curr().Type == Comment {
			d.next()
			continue
		}
		line, err := d.decodeScriptLine()
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	if d.curr().Type != EndScript {
		return d.unexpected()
	}
	d.next()
	if len(lines) == 0 {
		return fmt.Errorf("no script given")
	}
	var list []string
	for i := range lines {
		list = append(list, lines[i].Line)
	}
	lines[0].Line = strings.Join(list, "; ")
	cmd.Scripts = append(cmd.Scripts, lines[0])
	return nil
}

func decodeMacroRepeat(d *Decoder, cmd *Single) error {
	d.next()
	if d.curr().Type != BegList {
		return d.unexpected()
	}
	d.next()
	var list []Token
	for !d.done() && d.curr().Type != EndList {
		switch curr := d.curr(); curr.Type {
		case Quote:
			s, err := d.decodeQuote()
			if err != nil {
				return err
			}
			list = append(list, createToken(s, String))
		case String, Ident, Boolean:
			list = append(list, curr)
		case Variable:
			if d.peek().Type == Expand {
				vs, _ := d.locals.Resolve(curr.Literal)
				for i := range vs {
					list = append(list, createToken(vs[i], String))
				}
				d.next()
				break
			}
			list = append(list, curr)
		default:
			return d.unexpected()
		}
		d.next()
		switch d.curr().Type {
		case Comma:
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
	if d.curr().Type != BegScript {
		return d.unexpected()
	}
	d.next()
	var lines []Line
	for !d.done() && d.curr().Type != EndScript {
		if d.curr().Type == Comment {
			d.next()
			continue
		}
		line, err := d.decodeScriptLine()
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	if d.curr().Type != EndScript {
		return d.unexpected()
	}
	d.next()
	for i := range list {
		if list[i].IsVariable() {
			list[i].Literal = fmt.Sprintf("$%s", list[i].Literal)
		}
		var (
			iter  = strconv.Itoa(i + 1)
			iter0 = strconv.Itoa(i)
		)
		for _, n := range lines {
			n.Line = strings.ReplaceAll(n.Line, macroVar, list[i].Literal)
			n.Line = strings.ReplaceAll(n.Line, macroIter, iter)
			n.Line = strings.ReplaceAll(n.Line, macroIter0, iter0)
			cmd.Scripts = append(cmd.Scripts, n)
		}
	}
	return nil
}
