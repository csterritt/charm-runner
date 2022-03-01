package circular_buffer

import "sync"

type CircularBuffer struct {
	max      int
	num      int
	nextSlot int
	strings  []string
	lock     sync.Mutex
}

func MakeCircularBuffer(size int) CircularBuffer {
	return CircularBuffer{
		max:      size,
		num:      0,
		nextSlot: 0,
		strings:  make([]string, size),
	}
}

func (cb *CircularBuffer) AddString(s string) {
	cb.lock.Lock()
	defer cb.lock.Unlock()

	if cb.num != cb.max {
		cb.strings[cb.nextSlot] = s
		cb.nextSlot += 1
		cb.num += 1
		if cb.num == cb.max {
			cb.nextSlot = 0
		}
	} else {
		cb.strings[cb.nextSlot] = s
		cb.nextSlot = (cb.nextSlot + 1) % cb.max
	}
}

func (cb *CircularBuffer) Iter() <-chan string {
	ch := make(chan string)
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
