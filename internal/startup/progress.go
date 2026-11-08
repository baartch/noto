// Package startup defines progress events emitted during session initialization.
package startup

// Event describes one startup phase. Total is -1 while its size is unknown.
type Event struct {
	Phase string
	State string
	Done  int
	Total int
}

// Reporter receives a startup event; a nil reporter is permitted.
type Reporter func(Event)

// Send emits an event if a reporter is configured.
func (r Reporter) Send(event Event) {
	if r != nil {
		r(event)
	}
}
