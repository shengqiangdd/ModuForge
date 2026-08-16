package agent

import (
	"sync"
)

// FileLock provides per-file locking to prevent race conditions.
type FileLock struct {
	locks sync.Map // path -> *sync.Mutex
}

// fileLock is a per-file mutex wrapper.
type fileLock struct {
	mu sync.Mutex
}

// Lock acquires the lock for a specific file path.
func (fl *FileLock) Lock(path string) {
	val, _ := fl.locks.LoadOrStore(path, &fileLock{})
	l := val.(*fileLock)
	l.mu.Lock()
}

// Unlock releases the lock for a specific file path.
func (fl *FileLock) Unlock(path string) {
	if val, ok := fl.locks.Load(path); ok {
		l := val.(*fileLock)
		l.mu.Unlock()
	}
}

