package util

import "testing"

func BenchmarkIdGen_Get(b *testing.B) {
	idGen := NewIdGen("okex")
	for i := 0; i < b.N; i++ {
		b.Log(idGen.Get())
	}
}
