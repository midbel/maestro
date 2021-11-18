package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type Environment interface {
	Resolve(string) ([]string, error)
	Define(string, []string) error
	Delete(string)
}

type Expander interface {
	Expand(Environment) ([]string, error)
}

type Executer interface {
	Execute(Environment) error
}

type ExecAnd struct {
	Left  Executer
	Right Executer
}

func (a ExecAnd) Execute(env Environment) error {
	if err := a.Left.Execute(env); err != nil {
		return err
	}
	return a.Right.Execute(env)
}

type ExecOr struct {
	Left  Executer
	Right Executer
}

func (o ExecOr) Execute(env Environment) error {
	err := o.Left.Execute(env)
	if err == nil {
		return err
	}
	return o.Right.Execute(env)
}

type ExecPipe struct {
	Left  Executer
	Right Executer
	Both  bool
}

func (p ExecPipe) Execute(env Environment) error {
	return nil
}

type ExecList struct {
	List []Executer
}

func (i ExecList) Execute(env Environment) error {
	return nil
}

type ExecSimple struct {
	Words []Expander
	In    Expander
	Out   Expander
	Err   Expander
}

func (s ExecSimple) Execute(env Environment) error {
	return nil
}

type ExpandAssign struct {
	Ident string
	Words []Expander
}

func (a ExpandAssign) Execute(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandWord struct {
	Literal string
}

func (w ExpandWord) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandMulti struct {
	List []Expander
}

func (m ExpandMulti) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandVariable struct {
	Ident string
}

func (v ExpandVariable) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandLength struct {
	Ident string
}

func (v ExpandLength) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandReplace struct {
	Ident string
	From  string
	To    string
	What  rune
}

func (v ExpandReplace) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandTrim struct {
	Ident string
	Trim  string
	What  rune
}

func (v ExpandTrim) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandSlice struct {
	Ident string
	From  int
	To    int
}

func (v ExpandSlice) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandLower struct {
	Ident string
	All   bool
}

func (v ExpandLower) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandUpper struct {
	Ident string
	All   bool
}

func (v ExpandUpper) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandValIfUnset struct {
	Ident string
	Str   string
}

func (v ExpandValIfUnset) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandSetValIfUnset struct {
	Ident string
	Str   string
}

func (v ExpandSetValIfUnset) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandValIfSet struct {
	Ident string
	Str   string
}

func (v ExpandValIfSet) Expand(env Environment) ([]string, error) {
	return nil, nil
}

type ExpandExitIfUnset struct {
	Ident string
	Str   string
}

func (v ExpandExitIfUnset) Expand(env Environment) ([]string, error) {
	return nil, nil
}

func main() {
	list := []string{
		`maestro -f "maestro.mf" all $file foo-"test"-bar # comment`,
		`echo "string with a $variable in middle"`,
		`echo test-$variable-test`,
		`echo ${var}`,
		`echo ${var:-val}`,
		`echo ${var:=val}`,
		`echo ${var:+val}`,
		`echo ${var:?message}`,
		`echo ${#length}`,
		`echo ${var/from/to}`,
		`echo ${var//from/to}`,
		`echo ${var/%replace/suffix}`,
		`echo ${var/#replace/prefix}`,
		`echo ${var:0:2}`,
		`echo ${var%suffix}`,
		`echo ${var#prefix}`,
		`echo ${var%%long-suffix}`,
		`echo ${var##long-prefix}`,
		`echo ${lower-first,}`,
		`echo ${lower-all,,}`,
		`echo ${upper-first^}`,
		`echo ${upper-all^^}`,
		`echo first && echo second`,
		`echo first || echo second`,
		`echo first | echo -`,
		// `echo | echo; echo && echo || echo; echo`,
		// `echo < file.txt`,
		// `echo > file.txt`,
		// `echo >> file.txt`,
	}

	for i := range list {
		if i > 0 {
			fmt.Println("---")
		}
		scan := Scan(strings.NewReader(list[i]))
		for {
			tok := scan.Scan()
			fmt.Println(tok)
			if tok.Type == EOF || tok.Type == Invalid {
				break
			}
		}
	}
	fmt.Println("===")
	fmt.Println("===")
	for i := range list {
		if i > 0 {
			fmt.Println("---")
		}
		p := NewParser(strings.NewReader(list[i]))
		ex, err := p.Parse()
		if err != nil {
			fmt.Println(list[i], err)
			continue
		}
		fmt.Println(">>", list[i])
		fmt.Printf("%#v\n", ex)
	}
}

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	quoted bool
}

func NewParser(r io.Reader) *Parser {
	p := Parser{
		scan: Scan(r),
	}
	p.next()
	p.next()

	return &p
}

func (p *Parser) Parse() (Executer, error) {
	var list ExecList
	for !p.done() {
		ex, err := p.parse()
		if err != nil {
			return nil, err
		}
		list.List = append(list.List, ex)
	}
	var ex Executer = list
	if len(list.List) == 1 {
		ex = list.List[0]
	}
	return ex, nil
}

func (p *Parser) parse() (Executer, error) {
	var ex ExecSimple
	for !p.done() {
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case List, Comment:
			p.next()
			return ex, nil
		case And:
			return p.parseAnd(ex)
		case Or:
			return p.parseOr(ex)
		case Pipe, PipeBoth:
			return p.parsePipe(ex)
		case BegExp, Variable, Quote, Literal:
			next, err = p.parseWords()
		default:
			err = p.unexpected()
		}
		if err != nil {
			return nil, err
		}
		ex.Words = append(ex.Words, next)
	}
	return ex, nil
}

func (p *Parser) parsePipe(left Executer) (Executer, error) {
	both := p.curr.Type == PipeBoth
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return ExecPipe{
		Left:  left,
		Right: right,
		Both:  both,
	}, nil
}

func (p *Parser) parseAnd(left Executer) (Executer, error) {
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return ExecAnd{
		Left:  left,
		Right: right,
	}, nil
}

func (p *Parser) parseOr(left Executer) (Executer, error) {
	p.next()
	right, err := p.parse()
	if err != nil {
		return nil, err
	}
	return ExecOr{
		Left:  left,
		Right: right,
	}, nil
}

func (p *Parser) parseWords() (Expander, error) {
	var list ExpandMulti
	for !p.done() {
		if p.curr.Eow() {
			if !p.curr.IsSequence() {
				p.next()
			}
			break
		}
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case Literal:
			next, err = p.parseLiteral()
		case Variable:
			next, err = p.parseVariable()
		case Quote:
			next, err = p.parseQuote()
		case BegExp:
			next, err = p.parseExpansion()
		default:
			err = p.unexpected()
		}
		if err != nil {
			return nil, err
		}
		list.List = append(list.List, next)
	}
	var ex Expander = list
	if len(list.List) == 1 {
		ex = list.List[0]
	}
	return ex, nil
}

func (p *Parser) parseQuote() (ExpandMulti, error) {
	p.enterQuote()

	p.next()
	var ex ExpandMulti
	for !p.done() {
		if p.curr.Type == Quote {
			break
		}
		var (
			next Expander
			err  error
		)
		switch p.curr.Type {
		case Literal:
			next, err = p.parseLiteral()
		case Variable:
			next, err = p.parseVariable()
		case BegExp:
			next, err = p.parseExpansion()
		default:
			err = p.unexpected()
		}
		if err != nil {
			return ex, err
		}
		ex.List = append(ex.List, next)
	}
	if p.curr.Type != Quote {
		return ex, p.unexpected()
	}
	p.leaveQuote()
	p.next()
	return ex, nil
}

func (p *Parser) parseLiteral() (ExpandWord, error) {
	ex := ExpandWord{
		Literal: p.curr.Literal,
	}
	p.next()
	return ex, nil
}

func (p *Parser) parseExpansion() (Expander, error) {
	p.next()
	if p.curr.Type == Length {
		p.next()
		if p.curr.Type != Literal {
			return nil, p.unexpected()
		}
		ex := ExpandLength{
			Ident: p.curr.Literal,
		}
		p.next()
		if p.curr.Type != EndExp {
			return nil, p.unexpected()
		}
		p.next()
		return ex, nil
	}
	if p.curr.Type != Literal {
		return nil, p.unexpected()
	}
	ident := p.curr
	p.next()
	var (
		ex  Expander
		err error
	)
	switch p.curr.Type {
	case EndExp:
		ex = ExpandVariable{
			Ident: ident.Literal,
		}
	case Slice:
		e := ExpandSlice{
			Ident: ident.Literal,
		}
		p.next()
		e.From = 0 // p.curr.Literal
		p.next()
		if p.curr.Type != Slice {
			err = p.unexpected()
			break
		}
		p.next()
		e.To = 0 // p.curr.Literal
		p.next()
		ex = e
	case TrimSuffix, TrimSuffixLong, TrimPrefix, TrimPrefixLong:
		e := ExpandTrim{
			Ident: ident.Literal,
			What:  p.curr.Type,
		}
		p.next()
		e.Trim = p.curr.Literal
		p.next()
		ex = e
	case Replace, ReplaceAll, ReplacePrefix, ReplaceSuffix:
		e := ExpandReplace{
			Ident: ident.Literal,
			What:  p.curr.Type,
		}
		p.next()
		e.From = p.curr.Literal
		p.next()
		if p.curr.Type != Replace {
			err = p.unexpected()
			break
		}
		p.next()
		e.To = p.curr.Literal
		p.next()
		ex = e
	case Lower, LowerAll:
		e := ExpandLower{
			Ident: ident.Literal,
			All:   p.curr.Type == LowerAll,
		}
		p.next()
		ex = e
	case Upper, UpperAll:
		e := ExpandUpper{
			Ident: ident.Literal,
			All:   p.curr.Type == UpperAll,
		}
		p.next()
		ex = e
	case ValIfUnset:
		e := ExpandValIfUnset{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	case SetValIfUnset:
		e := ExpandSetValIfUnset{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	case ValIfSet:
		e := ExpandValIfSet{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	case ExitIfUnset:
		e := ExpandExitIfUnset{
			Ident: ident.Literal,
		}
		p.next()
		e.Str = p.curr.Literal
		p.next()
		ex = e
	default:
		err = p.unexpected()
	}
	if err != nil {
		return nil, err
	}
	if p.curr.Type != EndExp {
		return nil, p.unexpected()
	}
	p.next()
	return ex, nil
}

func (p *Parser) parseVariable() (ExpandVariable, error) {
	ex := ExpandVariable{
		Ident: p.curr.Literal,
	}
	p.next()
	return ex, nil
}

func (p *Parser) enterQuote() {
	p.quoted = true
}

func (p *Parser) leaveQuote() {
	p.quoted = false
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

func (p *Parser) done() bool {
	return p.curr.Type == EOF
}

func (p *Parser) unexpected() error {
	return fmt.Errorf("unexpected token %s [%s]", p.curr, p.peek)
}

const (
	zero       = 0
	space      = ' '
	tab        = '\t'
	squote     = '\''
	dquote     = '"'
	dollar     = '$'
	pound      = '#'
	percent    = '%'
	slash      = '/'
	comma      = ','
	colon      = ':'
	minus      = '-'
	plus       = '+'
	question   = '?'
	underscore = '_'
	lcurly     = '{'
	rcurly     = '}'
	equal      = '='
	caret      = '^'
	ampersand  = '&'
	pipe       = '|'
	semicolon  = ';'
	langle     = '<'
	rangle     = '>'
)

const (
	EOF = -(iota + 1)
	Blank
	Literal
	Quote
	Comment
	Variable
	BegExp
	EndExp
	List
	Pipe
	PipeBoth
	And
	Or
	RedirectIn
	RedirectOut
	AppendOut
	Length         // ${#var}
	Slice          // ${var:from:to}
	Replace        // ${var/from/to}
	ReplaceAll     // ${var//from/to}
	ReplaceSuffix  // ${var/%from/to}
	ReplacePrefix  // ${var/#from/to}
	TrimSuffix     // ${var%suffix}
	TrimSuffixLong // ${var%%suffix}
	TrimPrefix     // ${var#suffix}
	TrimPrefixLong // ${var##suffix}
	Lower          // ${var,}
	LowerAll       // ${var,,}
	Upper          // ${var^}
	UpperAll       // ${var^^}
	ValIfUnset     // ${var:-val}
	SetValIfUnset  // ${var:=val}
	ValIfSet       // ${var:+val}
	ExitIfUnset    // ${var:?val}
	Invalid
)

type Token struct {
	Literal string
	Type    rune
}

func (t Token) IsSequence() bool {
	switch t.Type {
	case And, Or, List, Pipe, PipeBoth:
		return true
	default:
		return false
	}
}

func (t Token) Eow() bool {
	return t.Type == Comment || t.Type == EOF || t.Type == Blank || t.IsSequence()
}

func (t Token) String() string {
	var prefix string
	switch t.Type {
	case EOF:
		return "<eof>"
	case Blank:
		return "<blank>"
	case Quote:
		return "<quote>"
	case And:
		return "<and>"
	case Or:
		return "<or>"
	case Pipe:
		return "<pipe>"
	case PipeBoth:
		return "<pipe-both>"
	case List:
		return "<list>"
	case BegExp:
		return "<beg-expansion>"
	case EndExp:
		return "<end-expansion>"
	case Length:
		return "<length>"
	case Slice:
		return "<slice>"
	case Replace:
		return "<replace>"
	case ReplaceAll:
		return "<replace-all>"
	case ReplaceSuffix:
		return "<replace-suffix>"
	case ReplacePrefix:
		return "<replace-prefix>"
	case TrimSuffix:
		return "<trim-suffix>"
	case TrimSuffixLong:
		return "<trim-suffix-long>"
	case TrimPrefix:
		return "<trim-prefix>"
	case TrimPrefixLong:
		return "<trim-prefix-long>"
	case Lower:
		return "<lower>"
	case LowerAll:
		return "<lower-all>"
	case Upper:
		return "<upper>"
	case UpperAll:
		return "<upper-all>"
	case ValIfUnset:
		return "<val-if-unset>"
	case SetValIfUnset:
		return "<set-val-if-unset>"
	case ValIfSet:
		return "<val-if-set>"
	case ExitIfUnset:
		return "<exit-if-unset>"
	case RedirectIn:
		return "<redirect-in>"
	case RedirectOut:
		return "<redirect-out>"
	case AppendOut:
		return "<append-out>"
	case Variable:
		prefix = "variable"
	case Comment:
		prefix = "comment"
	case Literal:
		prefix = "literal"
	case Invalid:
		prefix = "invalid"
	default:
		prefix = "unknown"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}

var colonOps = map[rune]rune{
	minus:    ValIfUnset,
	plus:     ValIfSet,
	equal:    SetValIfUnset,
	question: ExitIfUnset,
}

var slashOps = map[rune]rune{
	slash:   ReplaceAll,
	percent: ReplaceSuffix,
	pound:   ReplacePrefix,
}

type Scanner struct {
	input []byte
	char  rune
	curr  int
	next  int

	str      bytes.Buffer
	quoted   bool
	expanded bool
}

func Scan(r io.Reader) *Scanner {
	buf, _ := io.ReadAll(r)
	s := Scanner{
		input: buf,
	}
	s.read()
	return &s
}

func (s *Scanner) Scan() Token {
	s.reset()
	var tok Token
	if s.char == zero || s.char == utf8.RuneError {
		tok.Type = EOF
		return tok
	}
	switch {
	case isOperator(s.char) && s.expanded:
		s.scanOperator(&tok)
	case isBlank(s.char) && !s.quoted:
		tok.Type = Blank
		s.skipBlank()
	case isSequence(s.char) && !s.quoted:
		s.scanSequence(&tok)
	case isRedirect(s.char) && !s.quoted:
		s.scanRedirect(&tok)
	case isDouble(s.char):
		tok.Type = Quote
		s.read()
		s.toggleQuote()
	case isSingle(s.char):
		s.scanQuote(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isVariable(s.char):
		s.scanVariable(&tok)
	default:
		s.scanLiteral(&tok)
	}
	return tok
}

func (s *Scanner) scanRedirect(tok *Token) {
	switch s.char {
	case langle:
		tok.Type = RedirectIn
	case rangle:
		tok.Type = RedirectOut
		if k := s.peek(); k == s.char {
			tok.Type = AppendOut
			s.read()
		}
	default:
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanSequence(tok *Token) {
	switch k := s.peek(); {
	case s.char == semicolon:
		tok.Type = List
	case s.char == ampersand && k == s.char:
		tok.Type = And
		s.read()
	case s.char == pipe && k == s.char:
		tok.Type = Or
		s.read()
	case s.char == pipe && k == ampersand:
		tok.Type = PipeBoth
		s.read()
	case s.char == pipe:
		tok.Type = Pipe
	default:
		tok.Type = Invalid
	}
	s.read()
	s.skipBlank()
}

func (s *Scanner) scanOperator(tok *Token) {
	if k := s.prev(); s.char == pound && k == lcurly {
		tok.Type = Length
		s.read()
		return
	}
	switch s.char {
	case rcurly:
		tok.Type = EndExp
		s.expanded = false
	case colon:
		tok.Type = Slice
		if t, ok := colonOps[s.peek()]; ok {
			s.read()
			tok.Type = t
		}
	case slash:
		tok.Type = Replace
		if t, ok := slashOps[s.peek()]; ok {
			s.read()
			tok.Type = t
		}
	case percent:
		tok.Type = TrimSuffix
		if k := s.peek(); k == percent {
			tok.Type = TrimSuffixLong
			s.read()
		}
	case pound:
		tok.Type = TrimPrefix
		if k := s.peek(); k == pound {
			tok.Type = TrimPrefixLong
			s.read()
		}
	case comma:
		tok.Type = Lower
		if k := s.peek(); k == comma {
			tok.Type = LowerAll
			s.read()
		}
	case caret:
		tok.Type = Upper
		if k := s.peek(); k == caret {
			tok.Type = UpperAll
			s.read()
		}
	default:
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanVariable(tok *Token) {
	s.read()
	if s.char == lcurly {
		tok.Type = BegExp
		s.expanded = true
		s.read()
		return
	}
	if !isLetter(s.char) {
		tok.Type = Invalid
		return
	}
	for isIdent(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Variable
	tok.Literal = s.string()
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.skipBlank()
	for !s.done() {
		s.write()
		s.read()
	}
	tok.Type = Comment
	tok.Literal = s.string()
}

func (s *Scanner) scanQuote(tok *Token) {
	s.read()
	for !isSingle(s.char) && !s.done() {
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
	if !isSingle(s.char) {
		tok.Type = Invalid
	}
}

func (s *Scanner) scanLiteral(tok *Token) {
	if s.quoted {
		s.scanQuoted(tok)
		return
	}
	for !s.done() && !s.stopLiteral(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
	s.skipBlankUntil(isSequence)
}

func (s *Scanner) scanQuoted(tok *Token) {
	for !s.done() {
		if isDouble(s.char) || isVariable(s.char) {
			break
		}
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.string()
}

func (s *Scanner) toggleQuote() {
	s.quoted = !s.quoted
}

func (s *Scanner) reset() {
	s.str.Reset()
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) string() string {
	return s.str.String()
}

func (s *Scanner) peek() rune {
	r, _ := utf8.DecodeRune(s.input[s.next:])
	return r
}

func (s *Scanner) prev() rune {
	r, _ := utf8.DecodeLastRune(s.input[:s.curr])
	return r
}

func (s *Scanner) read() {
	if s.curr >= len(s.input) {
		s.char = 0
		return
	}
	r, n := utf8.DecodeRune(s.input[s.next:])
	if r == utf8.RuneError {
		s.char = 0
		s.next = len(s.input)
	}
	s.char, s.curr, s.next = r, s.next, s.next+n
}

func (s *Scanner) done() bool {
	return s.char == zero || s.char == utf8.RuneError
}

func (s *Scanner) skipBlank() {
	for isBlank(s.char) {
		s.read()
	}
}

func (s *Scanner) skipBlankUntil(fn func(rune) bool) {
	if !isBlank(s.char) {
		return
	}
	var (
		curr = s.curr
		next = s.next
		char = s.char
	)
	s.skipBlank()
	if !fn(s.char) {
		s.curr = curr
		s.next = next
		s.char = char
	}
}

func (s *Scanner) stopLiteral(r rune) bool {
	if s.expanded && isOperator(r) {
		return true
	}
	ok := isBlank(s.char) || isSequence(s.char) || isDouble(s.char) || isVariable(s.char)
	return ok
}

func isBlank(r rune) bool {
	return r == space || r == tab
}

func isDouble(r rune) bool {
	return r == dquote
}

func isSingle(r rune) bool {
	return r == squote
}

func isVariable(r rune) bool {
	return r == dollar
}

func isComment(r rune) bool {
	return r == pound
}

func isIdent(r rune) bool {
	return isLetter(r) || isDigit(r) || r == underscore
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isOperator(r rune) bool {
	return r == caret || r == pound || r == colon || r == slash || r == percent || r == comma || r == slash || r == rcurly
}

func isSequence(r rune) bool {
	return r == ampersand || r == pipe || r == semicolon
}

func isRedirect(r rune) bool {
	return r == langle || r == rangle
}
