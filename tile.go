package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"go.etcd.io/bbolt"
)

type TileData struct {
	Coord Coord
	Zoom  int
	File  string
	Cache int64
}

func (m *Map) GetTile(c Coord, z int) (td *TileData) {
	m.db.View(func(tx *bbolt.Tx) error {
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		zoom := tiles.Bucket([]byte(strconv.Itoa(z)))
		if zoom == nil {
			return nil
		}
		tileraw := zoom.Get([]byte(c.Name()))
		if tileraw == nil {
			return nil
		}
		json.Unmarshal(tileraw, &td)
		return nil
	})
	return
}

func (m *Map) SaveTile(c Coord, z int, f string) {
	m.db.Update(func(tx *bbolt.Tx) error {
		tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
		if err != nil {
			return err
		}
		zoom, err := tiles.CreateBucketIfNotExists([]byte(strconv.Itoa(z)))
		if err != nil {
			return err
		}
		td := &TileData{
			Coord: c,
			Zoom:  z,
			File:  f,
			Cache: time.Now().UnixNano(),
		}
		raw, err := json.Marshal(td)
		if err != nil {
			return err
		}
		m.gridUpdates.send(td)
		return zoom.Put([]byte(c.Name()), raw)
	})
	return
}

type TileCache struct {
	X, Y, Z, T int
}

func (m *Map) watchGridUpdates(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := rw.(http.Flusher)

	if !ok {
		http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	c := make(chan *TileData, 5)

	m.gridUpdates.watch(c)

	tileCache := make([]TileCache, 0, 100)

	m.db.View(func(tx *bbolt.Tx) error {
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		return tiles.ForEach(func(k, v []byte) error {
			zoom := tiles.Bucket(k)
			return zoom.ForEach(func(tk, tv []byte) error {
				td := TileData{}
				json.Unmarshal(tv, &td)
				tileCache = append(tileCache, TileCache{
					X: td.Coord.X,
					Y: td.Coord.Y,
					Z: td.Zoom,
					T: int(td.Cache),
				})
				return nil
			})
		})
	})

	raw, _ := json.Marshal(tileCache)
	fmt.Fprint(rw, "data: ")
	rw.Write(raw)
	fmt.Fprint(rw, "\n\n")
	tileCache = tileCache[:0]
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case e := <-c:
			found := false
			for i := range tileCache {
				if tileCache[i].X == e.Coord.X && tileCache[i].Y == e.Coord.Y && tileCache[i].Z == e.Zoom {
					tileCache[i].T = int(e.Cache)
					found = true
				}
			}
			if !found {
				tileCache = append(tileCache, TileCache{
					X: e.Coord.X,
					Y: e.Coord.Y,
					Z: e.Zoom,
					T: int(e.Cache),
				})
			}
		case <-ticker.C:
			raw, _ := json.Marshal(tileCache)
			fmt.Fprint(rw, "data: ")
			rw.Write(raw)
			fmt.Fprint(rw, "\n\n")
			tileCache = tileCache[:0]
			flusher.Flush()
		}
	}
}

var tileRegex = regexp.MustCompile("([0-9]+)/([-0-9]+)_([-0-9]+).png")

func (m *Map) gridTile(rw http.ResponseWriter, req *http.Request) {
	tile := tileRegex.FindStringSubmatch(req.URL.Path)
	z, err := strconv.Atoi(tile[1])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	x, err := strconv.Atoi(tile[2])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	y, err := strconv.Atoi(tile[3])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	td := m.GetTile(Coord{X: x, Y: y}, z)

	rw.Header().Set("Cache-Control", "private immutable")

	http.ServeFile(rw, req, filepath.Join(m.gridStorage, td.File))
}
