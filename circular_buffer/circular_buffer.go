package circular_buffer

import "sync"

type StringSource int

const (
	StdOut StringSource = iota
	StdErr
)

type StringWithSource struct {
	Line string
	Typ  StringSource
}

type CircularBuffer struct {
	max      int
	num      int
	nextSlot int
	strings  []StringWithSource
	lock     sync.Mutex
}

func MakeCircularBuffer(size int) CircularBuffer {
	return CircularBuffer{
		max:      size,
		num:      0,
		nextSlot: 0,
		strings:  make([]StringWithSource, size),
	}
}

func (cb *CircularBuffer) AddStdoutString(s string) {
	cb.lock.Lock()
	defer cb.lock.Unlock()

	if cb.num != cb.max {
		cb.strings[cb.nextSlot].Line = s
		cb.strings[cb.nextSlot].Typ = StdOut
		cb.nextSlot += 1
		cb.num += 1
		if cb.num == cb.max {
			cb.nextSlot = 0
		}
	} else {
		cb.strings[cb.nextSlot].Line = s
		cb.strings[cb.nextSlot].Typ = StdOut
		cb.nextSlot = (cb.nextSlot + 1) % cb.max
	}
}

func (cb *CircularBuffer) AddStderrString(s string) {
	cb.lock.Lock()
	defer cb.lock.Unlock()

	if cb.num != cb.max {
		cb.strings[cb.nextSlot].Line = s
		cb.strings[cb.nextSlot].Typ = StdErr
		cb.nextSlot += 1
		cb.num += 1
		if cb.num == cb.max {
			cb.nextSlot = 0
		}
	} else {
		cb.strings[cb.nextSlot].Line = s
		cb.strings[cb.nextSlot].Typ = StdErr
		cb.nextSlot = (cb.nextSlot + 1) % cb.max
	}
}

func (cb *CircularBuffer) Iter() <-chan StringWithSource {
	ch := make(chan StringWithSource)
	go func() {
		cb.lock.Lock()
		defer cb.lock.Unlock()

		if cb.num != cb.max {
			for index := 0; index < cb.num; index += 1 {
				ch <- cb.strings[index]
			}
		} else {
			index := cb.nextSlot
			count := 0
			for count != cb.max {
				count += 1
				ch <- cb.strings[index]
				index = (index + 1) % cb.max
			}
		}

		close(ch)
	}()
	return ch
}
