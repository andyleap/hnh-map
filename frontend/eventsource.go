package main

import "syscall/js"

// EventSource is used to receive server-sent events.
//
// It connects to a server over HTTP and receives events in text/event-stream format without closing the connection.
type EventSource struct {
	o js.Value
}

// New creates a new EventSource object. It returns immediately, without waiting to connect.
func NewES(url string) *EventSource {
	es := js.Global().Get("EventSource").New(url)
	return &EventSource{
		o: es,
	}
}

func (es *EventSource) On(event string, f func(data string)) {
	es.o.Call("addEventListener", event, js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		f(args[0].Get("data").String())
		return nil
	}))
}
