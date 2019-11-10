package worker

import (
	"github.com/asticode/go-astibob"
	"github.com/asticode/go-astilog"
	"github.com/pkg/errors"
)

func (w *Worker) addUIMessageNames(m *astibob.Message) (err error) {
	// Parse payload
	var names []string
	if names, err = astibob.ParseUIMessageNamesAddPayload(m); err != nil {
		err = errors.Wrap(err, "index: parsing message payload failed")
		return
	}

	// Add message names
	w.mu.Lock()
	for _, n := range names {
		w.us[n] = true
	}
	w.mu.Unlock()
	return
}

func (w *Worker) deleteUIMessageNames(m *astibob.Message) (err error) {
	// Parse payload
	var names []string
	if names, err = astibob.ParseUIMessageNamesDeletePayload(m); err != nil {
		err = errors.Wrap(err, "index: parsing message payload failed")
		return
	}

	// Delete message names
	w.mu.Lock()
	for _, n := range names {
		delete(w.us, n)
	}
	w.mu.Unlock()
	return
}

func (w *Worker) sendMessageToUI(m *astibob.Message) (err error) {
	// Only send message from current worker
	if m.From.WorkerName() != w.name {
		return
	}

	// No UI requested this message
	w.mu.Lock()
	if _, ok := w.us[m.Name]; !ok {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	// Log
	astilog.Debugf("worker: sending %s message to ui", m.Name)

	// Write
	if err = w.cw.WriteJSON(m); err != nil {
		err = errors.Wrap(err, "worker: writing JSON message failed")
		return
	}
	return
}
