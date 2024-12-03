package main

import (
	"sync"
	"time"
)

type ConnFilter struct {
	maxAttempts int
	attempts    map[uint32]*RingBuffer
	mutex       *sync.Mutex
}

func NewConnFilter(maxAttempts int) *ConnFilter {
	f := &ConnFilter{
		maxAttempts: maxAttempts,
		attempts:    make(map[uint32]*RingBuffer),
		mutex:       &sync.Mutex{},
	}
	return f
}

func (f *ConnFilter) Tick() {
	f.mutex.Lock()

	cutoff := time.Now().Unix() - 60
	for ip, rb := range f.attempts {
		deleteEntry := true
		for _, t := range rb.GetAll() {
			if t > cutoff {
				deleteEntry = false
				break
			}
		}
		if deleteEntry {
			delete(f.attempts, ip)
		}
	}

	f.mutex.Unlock()
}

func (cf *ConnFilter) RecordAndGetConnAttempts(ip uint32) int {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()

	if _, exists := cf.attempts[ip]; !exists {
		cf.attempts[ip] = NewRingBuffer(cf.maxAttempts)
	}

	attempts := cf.attempts[ip]
	attempts.Add(time.Now().Unix())

	return attempts.Len()
}
