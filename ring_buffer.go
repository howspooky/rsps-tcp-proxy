package main

type RingBuffer struct {
	buffer []int64
	size   int
	head   int
	count  int
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]int64, size),
		size:   size,
	}
}

func (rb *RingBuffer) Add(value int64) {
	if rb.count == rb.size {
		rb.head = (rb.head + 1) % rb.size
	} else {
		rb.count = rb.count + 1
	}
	rb.buffer[(rb.head+rb.count-1)%rb.size] = value
}

func (rb *RingBuffer) Len() int {
	return rb.count
}

func (rb *RingBuffer) GetAll() []int64 {
	elements := make([]int64, rb.count)
	for i := 0; i < rb.count; i++ {
		elements[i] = rb.buffer[(rb.head+i)%rb.size]
	}
	return elements
}
