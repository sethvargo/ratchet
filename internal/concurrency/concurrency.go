package concurrency

import "runtime"

// DefaultConcurrency returns the default concurrency. By default, this is the
// number of CPU cores - 1.
func DefaultConcurrency(min int64) int64 {
	cpus := runtime.NumCPU() - 1
	if int64(cpus) < min {
		return min
	}
	return int64(cpus)
}
