package main

import (
	"sync"
	"syscall/js"
)

var L js.Value

func initializeLeaflet() {
	doc := js.Global().Get("document")

	// Load leaflet CSS.
	link := doc.Call("createElement", "link")
	link.Set("href", "https://unpkg.com/leaflet@1.5.1/dist/leaflet.css")
	link.Set("type", "text/css")
	link.Set("rel", "stylesheet")
	doc.Get("head").Call("appendChild", link)

	// Load leaflet javascript.
	script := doc.Call("createElement", "script")
	script.Set("src", "https://unpkg.com/leaflet@1.5.1/dist/leaflet.js")
	doc.Get("head").Call("appendChild", script)

	var wg sync.WaitGroup
	wg.Add(2)
	var callback js.Func
	callback = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		L = js.Global().Get("L")
		callback.Release()
		wg.Done()
		script2 := doc.Call("createElement", "script")
		script2.Set("src", "/js/SmartTileLayer.js")
		doc.Get("head").Call("appendChild", script2)
		var callback2 js.Func
		callback2 = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			callback2.Release()
			wg.Done()
			return nil
		})
		script2.Set("onreadystatechange", callback2)
		script2.Set("onload", callback2)
		return nil
	})
	script.Set("onreadystatechange", callback)
	script.Set("onload", callback)

	wg.Wait()
}
