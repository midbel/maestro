package scan

type state int8

const (
	stateDefault state = 1 << iota
	stateQuote
	stateScript
)

func (s state) String() string {
	switch s {
	default:
		return "<other>"
	case stateDefault:
		return "<default>"
	case stateQuote:
		return "<quote>"
	case stateScript:
		return "<script>"
	}
}

type stack []state

func emptyStack() *stack {
	var s stack
	return &s
}

func (s *stack) push(st state) {
	*s = append(*s, st)
}

func (s *stack) pop() {
	x := *s
	*s = x[:len(x)-1]
}

func (s *stack) toggle() {
	if !s.isQuote() {
		s.push(stateQuote)
	} else {
		s.pop()
	}
}

func (s *stack) enter() {
	s.push(stateScript)
}

func (s *stack) leave() {
	if !s.isScript() {
		return
	}
	s.pop()
}

func (s *stack) skipBlank() bool {
	return s.isDefault() || s.isScript()
}

func (s *stack) isDefault() bool {
	return s.last() == stateDefault
}

func (s *stack) isQuote() bool {
	return s.last() == stateQuote
}

func (s *stack) isScript() bool {
	return s.last() == stateScript
}

func (s *stack) last() state {
	if len(*s) == 0 {
		return stateDefault
	}
	x := *s
	return x[len(x)-1]
}