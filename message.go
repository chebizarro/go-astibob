package astibob

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Identifier types
const (
	AbilityIdentifierType = "ability"
	IndexIdentifierType   = "index"
	UIIdentifierType      = "ui"
	WorkerIdentifierType  = "worker"
)

// Message names
const (
	CmdAbilityStartMessage         = "cmd.ability.start"
	CmdAbilityStopMessage          = "cmd.ability.stop"
	CmdUIPingMessage               = "cmd.ui.ping"
	CmdWorkerRegisterMessage       = "cmd.worker.register"
	EventAbilityCrashedMessage     = "event.ability.crashed"
	EventAbilityStartedMessage     = "event.ability.started"
	EventAbilityStoppedMessage     = "event.ability.stopped"
	EventUIDisconnectedMessage     = "event.ui.disconnected"
	EventUIWelcomeMessage          = "event.ui.welcome"
	EventWorkerDisconnectedMessage = "event.worker.disconnected"
	EventWorkerRegisteredMessage   = "event.worker.registered"
	EventWorkerWelcomeMessage      = "event.worker.welcome"
)

type Message struct {
	From    Identifier      `json:"from"`
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload,omitempty"`
	To      *Identifier     `json:"to,omitempty"`
}

type Identifier struct {
	Name   *string         `json:"name,omitempty"`
	Type   string          `json:"type,omitempty"`
	Types  map[string]bool `json:"types,omitempty"`
	Worker *string         `json:"worker,omitempty"`
}

func (o *Identifier) match(i Identifier) bool {
	// Check type
	if o.Types != nil {
		if i.Types != nil {
			match := false
			for t := range i.Types {
				if _, ok := o.Types[t]; ok {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		} else {
			if _, ok := o.Types[i.Type]; !ok {
				return false
			}
		}
	} else {
		if i.Types != nil {
			if _, ok := i.Types[o.Type]; !ok {
				return false
			}
		} else if o.Type != i.Type {
			return false
		}
	}

	// Check name
	if o.Name != nil && (i.Name == nil || *o.Name != *i.Name) {
		return false
	}

	// Check worker
	if o.Worker != nil && (i.Worker == nil || *o.Worker != *i.Worker) {
		return false
	}
	return true
}

type WelcomeUI struct {
	Name    string   `json:"name"`
	Workers []Worker `json:"workers,omitempty"`
}

type Worker struct {
	Abilities []Ability `json:"abilities,omitempty"`
	Addr      string    `json:"addr,omitempty"`
	Name      string    `json:"name"`
}

type Ability struct {
	Metadata
	Status     string `json:"status"`
	UIHomepage string `json:"ui_homepage,omitempty"`
}

type Metadata struct {
	Description string `json:"description"`
	Name        string `json:"name"`
}

type Error struct {
	Message string `json:"message"`
}

func NewMessage() *Message {
	return &Message{}
}

func newMessage(from Identifier, to *Identifier, name string) *Message {
	m := NewMessage()
	m.From = from
	m.Name = name
	m.To = to
	return m
}

func NewCmdWorkerRegisterMessage(from Identifier, to *Identifier, w Worker) (m *Message, err error) {
	// Create message
	m = newMessage(from, to, CmdWorkerRegisterMessage)

	// Marshal payload
	if m.Payload, err = json.Marshal(w); err != nil {
		err = errors.Wrap(err, "astibob: marshaling payload failed")
		return
	}
	return
}

func ParseCmdWorkerRegisterPayload(m *Message) (w Worker, err error) {
	if err = json.Unmarshal(m.Payload, &w); err != nil {
		err = errors.Wrap(err, "astibob: unmarshaling failed")
		return
	}
	return
}

func NewEventAbilityCrashedMessage(from Identifier, to *Identifier) *Message {
	return newMessage(from, to, EventAbilityCrashedMessage)
}

func NewEventAbilityStartedMessage(from Identifier, to *Identifier) *Message {
	return newMessage(from, to, EventAbilityStartedMessage)
}

func NewEventAbilityStoppedMessage(from Identifier, to *Identifier) *Message {
	return newMessage(from, to, EventAbilityStoppedMessage)
}

func NewEventUIDisconnectedMessage(from Identifier, to *Identifier, name string) (m *Message, err error) {
	// Create message
	m = newMessage(from, to, EventUIDisconnectedMessage)

	// Marshal payload
	if m.Payload, err = json.Marshal(name); err != nil {
		err = errors.Wrap(err, "astibob: marshaling payload failed")
		return
	}
	return
}

func NewEventUIWelcomeMessage(from Identifier, to *Identifier, name string, ws []Worker) (m *Message, err error) {
	// Create message
	m = newMessage(from, to, EventUIWelcomeMessage)

	// Marshal payload
	if m.Payload, err = json.Marshal(WelcomeUI{
		Name:    name,
		Workers: ws,
	}); err != nil {
		err = errors.Wrap(err, "astibob: marshaling payload failed")
		return
	}
	return
}

func ParseEventUIDisconnectedPayload(m *Message) (name string, err error) {
	if err = json.Unmarshal(m.Payload, &name); err != nil {
		err = errors.Wrap(err, "astibob: unmarshaling failed")
		return
	}
	return
}

func NewEventWorkerDisconnectedMessage(from Identifier, to *Identifier, worker string) (m *Message, err error) {
	// Create message
	m = newMessage(from, to, EventWorkerDisconnectedMessage)

	// Marshal payload
	if m.Payload, err = json.Marshal(worker); err != nil {
		err = errors.Wrap(err, "astibob: marshaling payload failed")
		return
	}
	return
}

func ParseEventWorkerDisconnectedPayload(m *Message) (worker string, err error) {
	if err = json.Unmarshal(m.Payload, &worker); err != nil {
		err = errors.Wrap(err, "astibob: unmarshaling failed")
		return
	}
	return
}

func NewEventWorkerRegisteredMessage(from Identifier, to *Identifier, w Worker) (m *Message, err error) {
	// Create message
	m = newMessage(from, to, EventWorkerRegisteredMessage)

	// Marshal payload
	if m.Payload, err = json.Marshal(w); err != nil {
		err = errors.Wrap(err, "astibob: marshaling payload failed")
		return
	}
	return
}

func ParseEventWorkerRegisteredPayload(m *Message) (w Worker, err error) {
	if err = json.Unmarshal(m.Payload, &w); err != nil {
		err = errors.Wrap(err, "astibob: unmarshaling failed")
		return
	}
	return
}

func NewEventWorkerWelcomeMessage(from Identifier, to *Identifier, ws []Worker) (m *Message, err error) {
	// Create message
	m = newMessage(from, to, EventWorkerWelcomeMessage)

	// Marshal payload
	if m.Payload, err = json.Marshal(ws); err != nil {
		err = errors.Wrap(err, "astibob: marshaling payload failed")
		return
	}
	return
}

func ParseEventWorkerWelcomePayload(m *Message) (ws []Worker, err error) {
	if err = json.Unmarshal(m.Payload, &ws); err != nil {
		err = errors.Wrap(err, "astibob: unmarshaling failed")
		return
	}
	return
}
