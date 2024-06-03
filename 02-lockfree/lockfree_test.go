package lockfree

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
)

type datastorev1 struct {
	mu    sync.RWMutex
	store map[string]int32
}

func (d *datastorev1) read(k string) (v int32, ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok = d.store[k]
	return
}

func (d *datastorev1) write(k string, v int32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[k] = v
}

func (d *datastorev1) inc(k string, v int32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[k] += v
}

var keys []string

func init() {
	for i := 0; i < 1024; i++ {
		randomBytes := make([]byte, 128)
		_, err := rand.Read(randomBytes)
		if err != nil {
			fmt.Println("Error generating random bytes:", err)
			return
		}

		keys = append(keys, string(randomBytes))
	}
}

func BenchmarkV1(b *testing.B) {
	d := &datastorev1{
		sync.RWMutex{},
		make(map[string]int32),
	}

	for i := 0; i < 1024; i++ {
		d.store[keys[i]] = int32(rand.Intn(100))
	}

	b.Run("reads, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)
				go func() {
					_, _ = d.read(keys[rand.Intn(1024)])
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})

	b.Run("incs, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)

				go func() {
					d.inc(keys[rand.Intn(1024)], int32(10))
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})

	b.Run("writes, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)

				go func() {
					d.write(keys[rand.Intn(1024)], int32(100))
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})
}

type datastorev2 struct {
	mu    sync.RWMutex
	store map[string]*int32
}

func (d *datastorev2) read(k string) (v int32, ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ptrV, ok := d.store[k]
	if ok {
		v = atomic.LoadInt32(ptrV)
	}

	return
}

func (d *datastorev2) write(k string, v int32) {
	// If an entry is already present, utilize the read lock and add the entry
	d.mu.RLock()
	ptrV, ok := d.store[k]
	if ok {
		atomic.StoreInt32(ptrV, v)
	}
	d.mu.RUnlock()
	if ok {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[k] = &v
}

func (d *datastorev2) inc(k string, v int32) {
	// If an entry is already present, utilize the read lock and inc the entry
	d.mu.RLock()
	ptrV, ok := d.store[k]
	if ok {
		atomic.AddInt32(ptrV, v)
	}
	d.mu.RUnlock()
	if ok {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[k] = &v
}

func BenchmarkV2(b *testing.B) {
	d := &datastorev2{
		sync.RWMutex{},
		make(map[string]*int32),
	}

	for i := 0; i < 1024; i++ {
		v := int32(rand.Intn(100))
		d.store[keys[i]] = &v
	}

	b.Run("only reads, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)
				go func() {
					_, _ = d.read(keys[rand.Intn(1024)])
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})

	b.Run("only inc, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)
				go func() {
					d.inc(keys[rand.Intn(1024)], int32(10))
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})

	b.Run("writes, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)

				go func() {
					d.write(keys[rand.Intn(1024)], int32(100))
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})
}

type entry struct {
	k string
	v int32
}

type datastorev3 struct {
	store    atomic.Pointer[map[string]*int32]
	toupdate chan entry
}

func (d *datastorev3) read(k string) (v int32, ok bool) {
	m := d.store.Load()
	if m == nil {
		return
	}

	ptrV, ok := (*m)[k]
	if ok {
		v = atomic.LoadInt32(ptrV)
	}

	return
}

func (d *datastorev3) write(k string, v int32) (err error) {
	m := d.store.Load()
	if m == nil {
		return errors.New("map not present")
	}

	ptrV, ok := (*m)[k]
	if ok {
		atomic.StoreInt32(ptrV, v)
		return
	}

	// Non blocking send, if the size is beyond what can be buffered return error
	select {
	case d.toupdate <- entry{k, v}:
		return errors.New("Queued")
	default:
		return errors.New("Try again later")
	}
}

func (d *datastorev3) inc(k string, v int32) (err error) {
	m := d.store.Load()
	if m == nil {
		return errors.New("map not present")
	}

	ptrV, ok := (*m)[k]
	if ok {
		atomic.AddInt32(ptrV, v)
		return
	}

	// Non blocking send, if the size is beyond what can be buffered return error
	select {
	case d.toupdate <- entry{k, v}:
		return errors.New("Queued")
	default:
		return errors.New("Try again later")
	}
}

func BenchmarkV3(b *testing.B) {
	m := make(map[string]*int32)
	d := &datastorev3{}
	for i := 0; i < 1024; i++ {
		v := int32(rand.Intn(100))
		m[keys[i]] = &v
	}
	d.store.Store(&m)
	d.toupdate = make(chan entry, 256)

	b.Run("only reads, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)
				go func() {
					_, ok := d.read(keys[rand.Intn(1024)])
					if !ok {
						panic("asdfasdf")
					}
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})

	b.Run("only inc, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)
				go func() {
					d.inc(keys[rand.Intn(1024)], int32(10))
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})

	b.Run("writes, concurrency:256", func(b *testing.B) {
		wg := sync.WaitGroup{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < 256; j++ {
				wg.Add(1)

				go func() {
					d.write(keys[rand.Intn(1024)], int32(100))
					wg.Done()
				}()
			}
			wg.Wait()
		}
	})
}
