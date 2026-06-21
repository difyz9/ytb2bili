package workflow

import "sync"

type TaskRuntimeRegistry struct {
	mu      sync.Mutex
	cancels map[string]func()
}

func NewTaskRuntimeRegistry() *TaskRuntimeRegistry {
	return &TaskRuntimeRegistry{
		cancels: make(map[string]func()),
	}
}

func (r *TaskRuntimeRegistry) Register(videoID string, cancel func()) {
	if r == nil || videoID == "" || cancel == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[videoID] = cancel
}

func (r *TaskRuntimeRegistry) Cancel(videoID string) bool {
	if r == nil || videoID == "" {
		return false
	}
	r.mu.Lock()
	cancel, ok := r.cancels[videoID]
	if ok {
		delete(r.cancels, videoID)
	}
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (r *TaskRuntimeRegistry) Finish(videoID string) {
	if r == nil || videoID == "" {
		return
	}
	r.mu.Lock()
	delete(r.cancels, videoID)
	r.mu.Unlock()
}

func (r *TaskRuntimeRegistry) Has(videoID string) bool {
	if r == nil || videoID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.cancels[videoID]
	return ok
}
