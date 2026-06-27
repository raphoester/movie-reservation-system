package xcolls

type Set[T comparable] struct {
	m map[T]struct{}
}

var dummyVal = struct{}{}

func NewSet[T comparable](items ...T) *Set[T] {
	s := NewSetWithSize[T](len(items))
	for _, item := range items {
		s.Add(item)
	}
	return s
}

func NewSetWithSize[T comparable](n int) *Set[T] {
	if n < 0 {
		n = 0
	}
	return &Set[T]{m: make(map[T]struct{}, n)}
}

func (s *Set[T]) Add(val T) {
	s.m[val] = dummyVal
}

func (s *Set[T]) Contains(val T) bool {
	_, exists := s.m[val]
	return exists
}

func (s *Set[T]) ForEach(f func(T)) {
	for key := range s.m {
		f(key)
	}
}

// Difference returns a new set containing elements in the receiver that are not in the other set.
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := NewSetWithSize[T](len(s.m))
	for key := range s.m {
		if !other.Contains(key) {
			result.Add(key)
		}
	}
	return result
}

func (s *Set[T]) ToSlice() []T {
	result := make([]T, 0, len(s.m))
	for key := range s.m {
		result = append(result, key)
	}
	return result
}

func (s *Set[T]) Size() int {
	return len(s.m)
}

func (s *Set[T]) Empty() bool {
	return len(s.m) == 0
}
