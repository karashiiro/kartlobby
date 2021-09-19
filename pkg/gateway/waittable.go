package gateway

import "sync"

type WaitTable struct {
	table map[string]bool
	lock  *sync.Mutex
}

func NewWaitTable() *WaitTable {
	return &WaitTable{
		table: make(map[string]bool),
		lock:  &sync.Mutex{},
	}
}

// Returns true if the provided key is set. Always call this after LockUnlock().
func (w *WaitTable) IsSet(key string) bool {
	if _, ok := w.table[key]; ok {
		return true
	}

	return false
}

// Sets the provided key in the table, and returns a function to unset it.
func (w *WaitTable) SetUnset(key string) func() {
	w.table[key] = true
	return func() {
		delete(w.table, key)
	}
}

// Waits for this table's lock to be freed, locks the table, and returns a function to unlock it.
func (w *WaitTable) LockUnlock() func() {
	w.lock.Lock()
	return w.lock.Unlock
}
