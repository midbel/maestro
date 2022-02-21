package stack

type Stack[T any] struct {
	list []T
}

func New[T any]() Stack[T] {
	var stk Stack[T]
	return stk
}

func (s *Stack[T]) Len() int {
	return len(s.list)
}

func (s *Stack[T]) Pop() {
	n := s.Len() - 1
	if n < 0 {
		return
	}
	s.list = s.list[:n]
}

func (s *Stack[T]) Push(item T) {
	s.list = append(s.list, item)
}

func (s *Stack[T]) At(n int) T {
	var ret T
	if n >= s.Len() {
		return ret
	}
	return s.list[n]
}

func (s *Stack[T]) Curr() T {
	var ret T
	if s.Len() > 0 {
		ret = s.list[s.Len()-1]
	}
	return ret
}
