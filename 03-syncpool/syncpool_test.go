package syncpool

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

type jobV1 struct {
	msgCnt int
}

func (j *jobV1) producer(ctx context.Context) {
	for i := 0; i < j.msgCnt; i++ {
		// Strings are immutable, results in copy every time
		input := "some_"
		input += fmt.Sprint(1)
		input += fmt.Sprintf("_message_%d", 1)
		input += fmt.Sprintf("_%d", rand.Intn(1000))
		j.consumer(ctx, input)
	}
}

func (j *jobV1) consumer(ctx context.Context, msg string) {
	_ = msg
}

func (j *jobV1) start() {
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	for i := 0; i < 2048; i++ {
		wg.Add(1)
		go func() {
			j.producer(ctx)
			wg.Done()
		}()
	}

	wg.Wait()
	cancel()
}

func BenchmarkV1(t *testing.B) {
	v1 := jobV1{
		msgCnt: 32,
	}

	t.ResetTimer()
	t.Run("v1-run", func(t *testing.B) {
		for j := 0; j < t.N; j++ {
			v1.start()
		}
	})
}

type jobV2 struct {
	msgCnt int
}

func (j *jobV2) producer(ctx context.Context) {
	for i := 0; i < j.msgCnt; i++ {
		var b bytes.Buffer
		b.Write([]byte("some_"))
		b.Write([]byte(fmt.Sprint(1)))
		b.Write([]byte(fmt.Sprintf("_message_%d", 1)))
		b.Write([]byte(fmt.Sprintf("_%d", rand.Intn(1000))))
		j.consumer(ctx, b.Bytes())
	}
}

func (j *jobV2) consumer(ctx context.Context, msg []byte) {
	_ = msg
}

func (j *jobV2) start() {
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	for i := 0; i < 2048; i++ {
		wg.Add(1)
		go func() {
			j.producer(ctx)
			wg.Done()
		}()
	}

	wg.Wait()
	cancel()
}

func BenchmarkV2(t *testing.B) {
	v2 := jobV2{
		msgCnt: 32,
	}

	t.ResetTimer()
	t.Run("v2-run", func(t *testing.B) {
		for j := 0; j < t.N; j++ {
			v2.start()
		}
	})
}

// Introducnig syncpools https://pkg.go.dev/sync#Pool
var bufPool = sync.Pool{
	New: func() any {
		// The Pool's New function should generally only return pointer
		// types, since a pointer can be put into the return interface
		// value without an allocation:
		return new(bytes.Buffer)
	},
}

// v2 + pool
// https://pkg.go.dev/sync#Pool
type jobV3 struct {
	msgCnt int
}

func (j *jobV3) producer(ctx context.Context) {
	b := bufPool.Get().(*bytes.Buffer)
	for i := 0; i < j.msgCnt; i++ {
		b.Reset()
		b.Write([]byte("some_"))
		b.Write([]byte(fmt.Sprint(1)))
		b.Write([]byte(fmt.Sprintf("_message_%d", 1)))
		b.Write([]byte(fmt.Sprintf("_%d", rand.Intn(1000))))
		j.consumer(ctx, b.Bytes())
	}
	bufPool.Put(b)
}

func (j *jobV3) consumer(ctx context.Context, msg []byte) {
	_ = msg
}

func (j *jobV3) start() {
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	for i := 0; i < 2048; i++ {
		wg.Add(1)
		go func() {
			j.producer(ctx)
			wg.Done()
		}()
	}

	wg.Wait()
	cancel()
}

func BenchmarkV3(t *testing.B) {
	v3 := jobV3{
		msgCnt: 32,
	}

	t.ResetTimer()
	t.Run("v3-run", func(t *testing.B) {
		for j := 0; j < t.N; j++ {
			v3.start()
		}
	})
}
