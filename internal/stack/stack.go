package stack

type Stack[T any] struct {
	list []T
}

func (s *Stack[T]) Len() int {
	return len(s.list)
}

func (s *Stack[T]) Pop() {
	if len(s.list) == 0 {
		return
	}
	s.list = s.list[:s.Len()-1]
}

func (s *Stack[T]) Push(item T) {
	s.list = append(s.list, item)
}

func (s *Stack[T]) Curr() T {
	var ret T
	if s.Len() > 0 {
		ret = s.list[s.Len()-1]
	}
	return ret
}
