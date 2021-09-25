package gateway

type waitingPlayer struct {
	Addr string
}

type playerQueue struct {
	C chan *waitingPlayer

	nPlayers int
}

// newPlayerQueue constructs a new player queue and returns a pointer
// to it. The capacity of the queue must be provided.
func newPlayerQueue(maxCapacity int) *playerQueue {
	q := playerQueue{
		C: make(chan *waitingPlayer, maxCapacity),
	}

	return &q
}

// Push enqueues a waiting player, returning true if the operation was
// successful, and false otherwise.
func (q *playerQueue) Push(wp *waitingPlayer) bool {
	if q.nPlayers == cap(q.C) {
		return false
	}
	q.C <- wp
	q.nPlayers++
	return true
}

// Pop dequeues a waiting player, blocking until the dequeue is possible.
func (q *playerQueue) Pop() *waitingPlayer {
	return <-q.C
}
