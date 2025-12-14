package sets

import (
	"iter"
	"maps"
)

type Set[T comparable] struct {
	data map[T]struct{}
}

func New[T comparable]() Set[T] {
	return Set[T]{
		data: make(map[T]struct{}),
	}
}

func (s *Set[T]) Insert(item T) bool {
	_, exists := s.data[item]
	s.data[item] = struct{}{}
	return !exists
}

func Singleton[T comparable](item T) Set[T] {
	n := New[T]()
	_ = n.Insert(item)
	return n
}

func (s *Set[T]) Remove(item T) bool {
	_, exists := s.data[item]
	if exists {
		delete(s.data, item)
	}
	return exists
}

func (s Set[T]) Contains(item T) bool {
	_, exists := s.data[item]
	return exists
}

func (s Set[T]) Len() int {
	return len(s.data)
}

func (s Set[T]) IsEmpty() bool {
	return len(s.data) == 0
}

func (s *Set[T]) Clear() {
	s.data = make(map[T]struct{})
}

func (s Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.data {
			if !yield(item) {
				return
			}
		}
	}
}

func (s Set[T]) Clone() Set[T] {
	return Set[T]{
		data: maps.Clone(s.data),
	}
}

func (s Set[T]) Union(other Set[T]) iter.Seq[T] {
	if s.Len() >= other.Len() {
		return chain(s.All(), other.Difference(s))
	} else {
		return chain(other.All(), s.Difference(other))
	}
}

func chain[T any](seqs ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, seq := range seqs {
			for item := range seq {
				if !yield(item) {
					return
				}
			}
		}
	}
}

func (s Set[T]) Intersection(other Set[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.data {
			if other.Contains(item) {
				if !yield(item) {
					return
				}
			}
		}
	}
}

func (s Set[T]) Difference(other Set[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.data {
			if !other.Contains(item) {
				if !yield(item) {
					return
				}
			}
		}
	}
}

func (s Set[T]) SymmetricDifference(other Set[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.data {
			if !other.Contains(item) {
				if !yield(item) {
					return
				}
			}
		}
		for item := range other.data {
			if !s.Contains(item) {
				if !yield(item) {
					return
				}
			}
		}
	}
}

func (s Set[T]) IsSubset(other Set[T]) bool {
	for item := range s.data {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

func (s Set[T]) IsSuperset(other Set[T]) bool {
	return other.IsSubset(s)
}

func (s Set[T]) IsDisjoint(other Set[T]) bool {
	for item := range s.data {
		if other.Contains(item) {
			return false
		}
	}
	return true
}

func (s Set[T]) Equal(other Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for item := range s.data {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

func Collect[T comparable](seq iter.Seq[T]) Set[T] {
	result := New[T]()
	for item := range seq {
		result.Insert(item)
	}
	return result
}
