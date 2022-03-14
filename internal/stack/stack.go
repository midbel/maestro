package stack

type Stack[S any] struct {
	list []S
}

func New[S any]() Stack[S] {
	var stk Stack[S]
	return stk
}

func (s *Stack[S]) RotateLeft(n int) {
	if n >= s.Len() {
		return
	}
	s.rotate(n)
}

func (s *Stack[S]) RotateRight(n int) {
	if n >= s.Len() {
		return
	}
	s.rotate(s.Len() - n)
}

func (s *Stack[S]) rotate(n int) {
	s.list = append(s.list[n:], s.list[:n]...)
}

func (s *Stack[S]) Len() int {
	return len(s.list)
}

func (s *Stack[S]) Pop() {
	n := s.Len() - 1
	if n < 0 {
		return
	}
	s.list = s.list[:n]
}

func (s *Stack[S]) RemoveRight(n int) {
	if n < 0 || n >= s.Len() {
		return
	}
	s.list = append(s.list[:n], s.list[n+1:]...)
}

func (s *Stack[S]) RemoveLeft(n int) {
	if n < 0 || n >= s.Len() {
		return
	}
	n = s.Len() - n
	s.list = append(s.list[:n], s.list[n+1:]...)
}

func (s *Stack[S]) Push(item S) {
	s.list = append(s.list, item)
}

func (s *Stack[S]) At(n int) S {
	var ret S
	if n >= s.Len() {
		return ret
	}
	return s.list[n]
}

func (s *Stack[S]) Curr() S {
	var ret S
	if s.Len() > 0 {
		ret = s.list[s.Len()-1]
	}
	return ret
}
