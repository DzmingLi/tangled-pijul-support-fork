package sets

import (
	"math/rand"
	"reflect"
	"testing/quick"
)

func (_ Set[T]) Generate(rand *rand.Rand, size int) reflect.Value {
	s := New[T]()

	var zero T
	itemType := reflect.TypeOf(zero)

	for {
		if s.Len() >= size {
			break
		}

		item, ok := quick.Value(itemType, rand)
		if !ok {
			continue
		}

		if val, ok := item.Interface().(T); ok {
			s.Insert(val)
		}
	}

	return reflect.ValueOf(s)
}
