package dynamodb

type empty struct{}
type countingSemaphore chan empty

// A bare-bones counting semaphore implementation based on: http://www.golangpatterns.info/concurrency/semaphores
func newCountingSemaphore(size int) countingSemaphore {
	return make(countingSemaphore, size)
}

func (semaphore countingSemaphore) Acquire() {
	semaphore <- empty{}
}

func (semaphore countingSemaphore) Release() {
	<-semaphore
}
