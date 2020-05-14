package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go.etcd.io/bbolt"
)

type Config struct {
	Title string   `json:"title"`
	Auths []string `json:"auths"`
}

func (m *Map) getChars(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !s.Auths.Has(AUTH_MARKERS) {
		json.NewEncoder(rw).Encode([]interface{}{})
		return
	}
	chars := []Character{}
	m.chmu.RLock()
	defer m.chmu.RUnlock()
	for _, v := range m.characters {
		chars = append(chars, v)
	}
	json.NewEncoder(rw).Encode(chars)
}

func (m *Map) getMarkers(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !s.Auths.Has(AUTH_MARKERS) {
		json.NewEncoder(rw).Encode([]interface{}{})
		return
	}
	markers := []FrontendMarker{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("markers"))
		if b == nil {
			return nil
		}
		grid := b.Bucket([]byte("grid"))
		if grid == nil {
			return nil
		}
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		return grid.ForEach(func(k, v []byte) error {
			m := Marker{}
			json.Unmarshal(v, &m)
			graw := grids.Get([]byte(m.GridID))
			if graw == nil {
				return nil
			}
			g := GridData{}
			json.Unmarshal(graw, &g)
			markers = append(markers, FrontendMarker{
				Image:  m.Image,
				Hidden: m.Hidden,
				ID:     m.ID,
				Name:   m.Name,
				Map:    g.Map,
				Position: Position{
					X: m.Position.X + g.Coord.X*100,
					Y: m.Position.Y + g.Coord.Y*100,
				},
			})
			return nil
		})
	})
	json.NewEncoder(rw).Encode(markers)
}

func (m *Map) getMaps(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	maps := map[int]*MapInfo{}
	m.db.View(func(tx *bbolt.Tx) error {
		mapB := tx.Bucket([]byte("maps"))
		if maps == nil {
			return nil
		}
		return mapB.ForEach(func(k, v []byte) error {
			mapid, err := strconv.Atoi(string(k))
			if err != nil {
				return nil
			}
			mi := &MapInfo{}
			json.Unmarshal(v, &mi)
			if mi.Hidden {
				return nil
			}
			maps[mapid] = mi
			return nil
		})
	})
	json.NewEncoder(rw).Encode(maps)
}

func (m *Map) config(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	config := Config{
		Auths: s.Auths,
	}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			return nil
		}
		title := b.Get([]byte("title"))
		config.Title = string(title)
		return nil
	})
	json.NewEncoder(rw).Encode(config)
}
