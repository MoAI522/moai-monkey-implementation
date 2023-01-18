package object

func NewThreadPool() *ThreadPool {
	s := make(map[int64]Thread)
	return &ThreadPool{store: s, counter: 0}
}

type Thread struct {
	c     chan Object
	alive bool
}

type ThreadPool struct {
	store   map[int64]Thread
	counter int64
}

func (e *ThreadPool) Get(threadID int64) (chan Object, bool) {
	obj, ok := e.store[threadID]
	if !ok {
		return nil, false
	}
	if !obj.alive {
		return nil, false
	}
	return obj.c, ok
}

func (e *ThreadPool) Set(val chan Object) int64 {
	threadID := e.counter
	e.store[threadID] = Thread{c: val, alive: true}
	e.counter += 1
	return threadID
}
