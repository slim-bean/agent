package kubetail

import (
	"sync"
	"time"
)

type RollingAverageCalculator struct {
	mtx           sync.Mutex
	window        []time.Duration
	windowSize    int
	currentIndex  int
	prevTimestamp time.Time
}

func NewRollingAverageCalculator(windowSize int) *RollingAverageCalculator {
	return &RollingAverageCalculator{
		windowSize:   windowSize,
		window:       make([]time.Duration, windowSize),
		currentIndex: -1,
	}
}

func (r *RollingAverageCalculator) AddTimestamp(timestamp time.Time) {
	r.mtx.Lock()
	defer func() {
		r.prevTimestamp = timestamp
		r.mtx.Unlock()
	}()

	// First timestamp
	if r.currentIndex == -1 && r.prevTimestamp.Equal(time.Time{}) {
		return
	}

	r.currentIndex++
	if r.currentIndex >= r.windowSize {
		r.currentIndex = 0
	}

	r.window[r.currentIndex] = timestamp.Sub(r.prevTimestamp)

}

func (r *RollingAverageCalculator) GetAverage() time.Duration {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	var total time.Duration
	count := 0
	for _, v := range r.window {
		if v != 0 {
			total += v
			count++
		}
	}
	if count == 0 {
		return time.Duration(0)
	}
	return total / time.Duration(count)
}

func (r *RollingAverageCalculator) GetLast() time.Time {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	return r.prevTimestamp
}
