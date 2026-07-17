package concurrency

import "runtime"

// DefaultConcurrency returns the default concurrency. By default, this is the
// number of CPU cores - 1.
func DefaultConcurrency(minimum int64) int64 {
	cpus := int64(runtime.NumCPU() - 1)
	return max(cpus, minimum)
}
