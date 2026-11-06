package window

import (
	"sync"

	"github.com/rs/zerolog/log"
)

func (w *Window) AddMouseChangeCallback(cb func(uint, uint)) (callbackId int) {
	id := w.mouseChangeCallbackId
	w.mouseChangeCallbacks[id] = cb
	w.mouseChangeCallbackId++
	log.Print("mouse change callback added:", id)
	return id
}

func (w *Window) RemoveMouseChangeCallback(id int) {
	delete(w.mouseChangeCallbacks, id)
	log.Print("mouse change callback deleted:", id)
}

var (
	laterJobsMtx sync.Mutex
	laterJobs    []func()
)

// RunLater queues provided a job to be run in the next frame.
// Safe to call from any goroutine. Jobs queued while other jobs are running will be executed in the next frame.
func RunLater(job func()) {
	laterJobsMtx.Lock()
	laterJobs = append(laterJobs, job)
	laterJobsMtx.Unlock()
}

// takeLaterJobs atomically takes all queued jobs, so new jobs could be safely queued while executing taken ones.
func takeLaterJobs() []func() {
	laterJobsMtx.Lock()
	jobs := laterJobs
	laterJobs = nil
	laterJobsMtx.Unlock()
	return jobs
}

var repeatJobs []func()

// RunRepeat stores provided a job in a separate slice to run it in all other frames.
// Unlike the RunLater job will be executed all the time until the application shut down.
func RunRepeat(job func()) {
	repeatJobs = append(repeatJobs, job)
}
