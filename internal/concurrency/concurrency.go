package concurrency

import "runtime"

// DefaultConcurrency returns the default concurrency. By default, this is the
// number of CPU cores - 1.
func DefaultConcurrency(min uint64) uint64 {
	cpus := runtime.NumCPU() - 1
	if uint64(cpus) < min {
		return min
	}
	return uint64(cpus)
}
