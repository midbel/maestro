package scan

type scanState int8

const (
	scanDefault scanState = iota
	scanValue
	scanScript
	scanQuote
)

type scanstack struct {
	Stack stack[scanState]
}

func defaultStack() *scanstack {
	var s scanstack
	s.Stack = emptyStack[scanState]()
	s.Stack.Push(scanDefault)
	return &s
}

func (s *scanstack) Pop() {
	s.Stack.Pop()
}

func (s *scanstack) Push(state scanState) {
	s.Stack.Push(state)
}

func (s *scanstack) KeepBlank() bool {
	curr := s.Stack.Top()
	return curr == scanDefault || curr == scanValue
}

func (s *scanstack) Default() bool {
	return s.Stack.Top() == scanDefault
}

func (s *scanstack) Value() bool {
	return s.Stack.Top() == scanValue
}

func (s *scanstack) Quote() bool {
	return s.Stack.Top() == scanQuote
}

func (s *scanstack) ToggleQuote() {
	if s.Quote() {
		s.Stack.Pop()
		return
	}
	s.Stack.Push(scanQuote)
}

func (s *scanstack) Script() bool {
	return s.Stack.Top() == scanScript
}

func (s *scanstack) Curr() scanState {
	if s.Stack.Len() == 0 {
		return scanDefault
	}
	return s.Stack.Top()
}

func (s *scanstack) Prev() scanState {
	n := s.Stack.Len()
	n--
	n--
	if n >= 0 {
		return s.Stack.At(n)
	}
	return scanDefault
}

type stack[T any] struct {
	list []T
}

func emptyStack[T any]() stack[T] {
	var stk stack[T]
	return stk
}

func (s *stack[T]) Len() int {
	return len(s.list)
}

func (s *stack[T]) Pop() {
	n := s.Len() - 1
	if n < 0 {
		return
	}
	s.list = s.list[:n]
}

func (s *stack[T]) Push(item T) {
	s.list = append(s.list, item)
}

func (s *stack[T]) At(n int) T {
	var ret T
	if n >= s.Len() {
		return ret
	}
	return s.list[n]
}

func (s *stack[T]) Top() T {
	var ret T
	if s.Len() > 0 {
		ret = s.list[s.Len()-1]
	}
	return ret
}
