package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/asticode/go-astibob"
	"github.com/asticode/go-astilog"
	astiptr "github.com/asticode/go-astitools/ptr"
	astiworker "github.com/asticode/go-astitools/worker"
	"github.com/asticode/go-astiws"
	"github.com/pkg/errors"
)

type Options struct {
	Index  astibob.ServerOptions `toml:"index"`
	Server astibob.ServerOptions `toml:"server"`
}

type Worker struct {
	ch   *http.Client
	cw   *astiws.Client
	d    *astibob.Dispatcher
	ds   map[int]OnDone // On done callbacks indexed by message id
	id   int
	ls   map[string]map[string]map[string]bool // Worker's listenables indexed by worker --> runnable --> message
	md   *sync.Mutex                           // Locks ds
	mi   *sync.Mutex                           // Locks id
	ml   *sync.Mutex                           // Locks ls
	mo   *sync.Mutex                           // Locks ols
	mr   *sync.Mutex                           // Locks rs
	mu   *sync.Mutex                           // Locks us
	mw   *sync.Mutex                           // Locks ws
	name string
	o    Options
	ols  map[string]map[string]map[string]bool // Other workers listenables indexed by runnable --> worker --> message
	rs   map[string]astibob.Runnable
	us   map[string]bool // UI messages names indexed by message
	w    *astiworker.Worker
	ws   map[string]*worker
}

// New creates a new worker
func New(name string, o Options) (w *Worker) {
	// Create worker
	w = &Worker{
		ch:   &http.Client{},
		cw:   astiws.NewClient(astiws.ClientConfiguration{}),
		ds:   make(map[int]OnDone),
		ls:   make(map[string]map[string]map[string]bool),
		md:   &sync.Mutex{},
		mi:   &sync.Mutex{},
		ml:   &sync.Mutex{},
		mo:   &sync.Mutex{},
		mr:   &sync.Mutex{},
		mu:   &sync.Mutex{},
		mw:   &sync.Mutex{},
		name: name,
		o:    o,
		ols:  make(map[string]map[string]map[string]bool),
		rs:   make(map[string]astibob.Runnable),
		us:   make(map[string]bool),
		w:    astiworker.NewWorker(),
		ws:   make(map[string]*worker),
	}

	// Create dispatcher
	w.d = astibob.NewDispatcher(w.w.Context(), w.w.NewTask)

	// Add websocket message handler
	w.cw.SetMessageHandler(w.handleIndexMessage)

	// Add dispatcher handlers
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.ListenablesRegisterMessage)}, w.registerListenables)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.RunnableDoneMessage)}, w.doneMessage)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.RunnableStartMessage)}, w.startRunnableFromMessage)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.RunnableStopMessage)}, w.stopRunnableFromMessage)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.UIMessageNamesAddMessage)}, w.addUIMessageNames)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.UIMessageNamesDeleteMessage)}, w.deleteUIMessageNames)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.WorkerRegisteredMessage)}, w.registerWorker)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.WorkerDisconnectedMessage)}, w.unregisterWorker)
	w.d.On(astibob.DispatchConditions{Name: astiptr.Str(astibob.WorkerWelcomeMessage)}, w.finishRegistration)
	w.d.On(astibob.DispatchConditions{To: &astibob.Identifier{Type: astibob.IndexIdentifierType}}, w.sendMessageToIndex)
	w.d.On(astibob.DispatchConditions{To: &astibob.Identifier{Type: astibob.UIIdentifierType}}, w.sendMessageToUI)
	w.d.On(astibob.DispatchConditions{To: &astibob.Identifier{Types: map[string]bool{
		astibob.RunnableIdentifierType: true, // Example: Cmds
		astibob.WorkerIdentifierType:   true, // Example: Events
	}}}, w.sendMessageToWorker)
	return
}

// HandleSignals handles signals
func (w *Worker) HandleSignals() {
	w.w.HandleSignals()
}

// Wait waits for the index to be stopped
func (w *Worker) Wait() {
	w.w.Wait()
}

// On makes sure to handle messages with specific conditions
func (w *Worker) On(c astibob.DispatchConditions, h astibob.MessageHandler) {
	w.d.On(c, h)
}

// Close closes the worker properly
func (w *Worker) Close() error {
	// Close dispatcher
	w.d.Close()

	// Close client
	if w.cw != nil {
		if err := w.cw.Close(); err != nil {
			astilog.Error(errors.Wrap(err, "worker: closing client failed"))
		}
	}
	return nil
}

func (w *Worker) workerIdentifier() *astibob.Identifier {
	return astibob.NewWorkerIdentifier(w.name)
}

func (w *Worker) runnableIdentifier(name string) *astibob.Identifier {
	return astibob.NewRunnableIdentifier(name, w.name)
}

type worker struct {
	addr string
	mr   *sync.Mutex // Locks rs
	name string
	rs   map[string]astibob.RunnableMessage
}

func newWorker(i astibob.Worker) (w *worker) {
	// Create
	w = &worker{
		addr: i.Addr,
		mr:   &sync.Mutex{},
		name: i.Name,
		rs:   make(map[string]astibob.RunnableMessage),
	}

	// Loop through runnables
	for _, r := range i.Runnables {
		w.rs[r.Name] = r
	}
	return
}

func (w *worker) toMessage() (o astibob.Worker) {
	// Lock
	w.mr.Lock()
	defer w.mr.Unlock()

	// Create worker
	o = astibob.Worker{
		Addr: w.addr,
		Name: w.name,
	}

	// Loop through runnables
	for _, r := range w.rs {
		// Append
		o.Runnables = append(o.Runnables, r)
	}
	return
}

func (w *Worker) sendMessageToWorker(m *astibob.Message) (err error) {
	// Get from worker
	fw := m.From.WorkerName()

	// Only send message from the current worker
	if fw != w.name {
		return
	}

	// Invalid to
	if m.To == nil {
		err = errors.Wrap(err, "worker: no to")
		return
	}

	// Get workers
	var ws []*worker
	if tw := m.To.WorkerName(); tw != "" {
		// Only send message to other workers
		if tw == w.name {
			return
		}

		// Get worker
		w.mw.Lock()
		mw, ok := w.ws[tw]
		w.mw.Unlock()

		// No worker
		if !ok {
			err = fmt.Errorf("worker: worker %s doesn't exist", tw)
			return
		}

		// Append
		ws = append(ws, mw)
	} else {
		// Lock
		w.mw.Lock()

		// Loop through workers
		for _, mw := range w.ws {
			// Same worker
			if mw.name == w.name {
				continue
			}

			// Append
			ws = append(ws, mw)
		}

		// Unlock
		w.mw.Unlock()
	}

	// Loop through workers
	for _, mw := range ws {
		// Log
		astilog.Debugf("worker: sending message %s to worker %s", m.Name, mw.name)

		// Marshal
		var b []byte
		if b, err = json.Marshal(m); err != nil {
			err = errors.Wrap(err, "worker: marshaling failed")
			return
		}

		// Send request
		if err = w.sendRequestToWorker(http.MethodPost, fmt.Sprintf("%s/api/messages", mw.addr), bytes.NewReader(b)); err != nil {
			err = errors.Wrapf(err, "worker: sending request to worker %s failed", mw.name)
			return
		}
	}
	return
}

func (w *Worker) sendRequestToWorker(method, url string, body io.Reader) (err error) {
	// Create request
	var req *http.Request
	if req, err = http.NewRequest(method, url, body); err != nil {
		err = errors.Wrap(err, "worker: creating request failed")
		return
	}

	// Send request
	var resp *http.Response
	if resp, err = w.ch.Do(req); err != nil {
		err = errors.Wrap(err, "worker: sending request failed")
		return
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Unmarshal
		// We silence the error since there may not be an error message in the response
		var e astibob.Error
		json.NewDecoder(resp.Body).Decode(&e)

		// Log
		if e.Message != "" {
			err = fmt.Errorf("worker: response error message is %s", e.Message)
		} else {
			err = fmt.Errorf("worker: response status code is %d", resp.StatusCode)
		}
		return
	}
	return
}
