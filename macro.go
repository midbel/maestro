package maestro

import (
	"fmt"
	"strings"
)

const (
	macroRepeat   = "repeat"
	macroSequence = "sequence"
	macroVar      = "<var>"
)

func decodeMacroSequence(d *Decoder, cmd *Single) error {
	d.next()
	if d.curr().Type != BegScript {
		return d.unexpected()
	}
	d.next()

	var lines []Line
	for !d.done() {
		if d.curr().Type == EndScript {
			break
		}
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
	for !d.done() {
		if d.curr().Type == EndList {
			break
		}
		switch curr := d.curr(); {
		case curr.IsPrimitive() || curr.IsVariable():
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
	for !d.done() {
		if d.curr().Type == EndScript {
			break
		}
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
		for _, n := range lines {
			n.Line = strings.ReplaceAll(n.Line, macroVar, list[i].Literal)
			cmd.Scripts = append(cmd.Scripts, n)
		}
	}
	return nil
}
