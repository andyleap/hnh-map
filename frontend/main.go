package main

import (
	"encoding/json"
	"strconv"
	"syscall/js"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/style"
	router "marwan.io/vecty-router"
)

func main() {
	initializeLeaflet()
	vecty.SetTitle("HnH Auto Mapper Server")
	vecty.RenderBody(&PageView{})
}

// PageView is our main page component.
type PageView struct {
	vecty.Core
}

// Render implements the vecty.Component interface.
func (p *PageView) Render() vecty.ComponentOrHTML {
	return elem.Body(
		router.NewRoute("/", &MainView{}, router.NewRouteOpts{ExactMatch: true}),
		router.NewRoute("/map", &Map{}, router.NewRouteOpts{}),
	)
}

type MainView struct {
	vecty.Core
}

func (mv *MainView) Render() vecty.ComponentOrHTML {
	return elem.Div(
		vecty.Text("Main view"),
		router.Link("/map", "map", router.LinkOptions{}),
	)

}

type Map struct {
	vecty.Core
	mapDiv *vecty.HTML

	BaseMap js.Value

	MainLayer    js.Value
	OverlayLayer js.Value
	ES           *EventSource
}

func (m *Map) Render() vecty.ComponentOrHTML {
	if m.mapDiv == nil {
		m.mapDiv = elem.Div(
			vecty.Markup(
				style.Height(style.Size("100vh")),
				//style.Width(style.Size("100vw")),
			),
		)
	}
	return elem.Div(
		m.mapDiv,
		vecty.Text("Map"),
	)

}

type TileCache struct {
	M, X, Y, Z, T int
}

func (m *Map) Mount() {
	m.BaseMap = L.Call("map", m.mapDiv.Node(), map[string]interface{}{
		"minZoom": 1,
		"maxZoom": 6,
		"crs":     L.Get("CRS").Get("Simple"),
	})

	m.MainLayer = js.Global().Get("SmartTileLayer").New("/map/grids/{layer}/{z}/{x}_{y}.png?{cache}", map[string]interface{}{
		"minZoom":     1,
		"maxZoom":     6,
		"zoomOffset":  0,
		"zoomReverse": true,
		"tileSize":    100,
	})
	m.MainLayer.Set("invalidTile", "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	m.MainLayer.Set("layer", 2)
	m.MainLayer.Call("addTo", m.BaseMap)
	m.BaseMap.Call("setView", []interface{}{0, 0}, 1)

	m.ES = NewES("/map/api/updates")
	m.ES.On("message", func(data string) {
		updates := []TileCache{}
		json.Unmarshal([]byte(data), &updates)
		for _, u := range updates {
			m.MainLayer.Get("cache").Set(strconv.Itoa(u.M)+":"+strconv.Itoa(u.X)+":"+strconv.Itoa(u.Y)+":"+strconv.Itoa(u.Z), u.T)
			if m.MainLayer.Get("layer").Int() == u.M {
				m.MainLayer.Call("refresh", u.X, u.Y, u.Z)
			}
		}
	})
}
