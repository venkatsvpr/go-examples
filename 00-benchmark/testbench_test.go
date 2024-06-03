package testbench

import (
	"fmt"
	"testing"
)

func fibv1(n int) int {
	if n < 2 {
		return n
	}

	return fibv1(n-1) + fibv1(n-2)
}

func BenchmarkSimple(t *testing.B) {
	for i := 0; i < t.N; i++ {
		_ = fibv1(30)
	}
}

func benchmarkFibFn(t *testing.B, val int, fn func(int) int) {
	for i := 0; i < t.N; i++ {
		_ = fn(val)
	}
}

func BenchmarkFibMultiple(t *testing.B) {
	val := 64
	for i := 1; i <= val; i *= 2 {
		t.Run(fmt.Sprintf("val-%d", i), func(t *testing.B) {
			benchmarkFibFn(t, i, fibv1)
		})
	}
}

var cache map[int]int = make(map[int]int, 256)

func fibv2(n int) int {
	if n < 2 {
		return n
	}

	out, ok := cache[n]
	if ok {
		return out
	}

	cache[n] = fibv2(n-1) + fibv2(n-2)
	return cache[n]
}

func BenchmarkFibCached(t *testing.B) {
	val := 64
	for i := 1; i <= val; i *= 2 {
		t.Run(fmt.Sprintf("val-cached-%d", i), func(t *testing.B) {
			benchmarkFibFn(t, i, fibv2)
		})
	}
}
