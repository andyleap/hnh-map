package main

type Evented struct {
	handlers map[string]func()
}
