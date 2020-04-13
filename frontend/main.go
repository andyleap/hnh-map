package main

import (
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
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
}

func (m *Map) Render() vecty.ComponentOrHTML {
	return elem.Div(
		vecty.Text("Map"),
	)

}

func (m *Map) Mount() {
	L.
}