package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"go.etcd.io/bbolt"
)

type TileData struct {
	MapID int
	Coord Coord
	Zoom  int
	File  string
	Cache int64
}

func (m *Map) GetTile(mapid int, c Coord, z int) (td *TileData) {
	m.db.View(func(tx *bbolt.Tx) error {
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		mapb := tiles.Bucket([]byte(strconv.Itoa(mapid)))
		if mapb == nil {
			return nil
		}
		zoom := mapb.Bucket([]byte(strconv.Itoa(z)))
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

func (m *Map) SaveTile(mapid int, c Coord, z int, f string, t int64) {
	m.db.Update(func(tx *bbolt.Tx) error {
		tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
		if err != nil {
			return err
		}
		mapb, err := tiles.CreateBucketIfNotExists([]byte(strconv.Itoa(mapid)))
		if err != nil {
			return err
		}
		zoom, err := mapb.CreateBucketIfNotExists([]byte(strconv.Itoa(z)))
		if err != nil {
			return err
		}
		td := &TileData{
			MapID: mapid,
			Coord: c,
			Zoom:  z,
			File:  f,
			Cache: t,
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

func (m *Map) reportMerge(from, to int, shift Coord) {
	m.mergeUpdates.send(&Merge{
		From:  from,
		To:    to,
		Shift: shift,
	})
}

type TileCache struct {
	M, X, Y, Z, T int
}

func (m *Map) watchGridUpdates(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := rw.(http.Flusher)

	if !ok {
		http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	c := make(chan *TileData, 1000)
	mc := make(chan *Merge, 5)

	m.gridUpdates.watch(c)
	m.mergeUpdates.watch(mc)

	tileCache := make([]TileCache, 0, 100)

	m.db.View(func(tx *bbolt.Tx) error {
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		return tiles.ForEach(func(mk, mv []byte) error {
			mapb := tiles.Bucket(mk)
			if mapb == nil {
				return nil
			}
			return mapb.ForEach(func(k, v []byte) error {
				zoom := mapb.Bucket(k)
				if zoom == nil {
					return nil
				}
				return zoom.ForEach(func(tk, tv []byte) error {
					td := TileData{}
					json.Unmarshal(tv, &td)
					tileCache = append(tileCache, TileCache{
						M: td.MapID,
						X: td.Coord.X,
						Y: td.Coord.Y,
						Z: td.Zoom,
						T: int(td.Cache),
					})
					return nil
				})
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
		case e, ok := <-c:
			if !ok {
				return
			}
			found := false
			for i := range tileCache {
				if tileCache[i].M == e.MapID && tileCache[i].X == e.Coord.X && tileCache[i].Y == e.Coord.Y && tileCache[i].Z == e.Zoom {
					tileCache[i].T = int(e.Cache)
					found = true
				}
			}
			if !found {
				tileCache = append(tileCache, TileCache{
					M: e.MapID,
					X: e.Coord.X,
					Y: e.Coord.Y,
					Z: e.Zoom,
					T: int(e.Cache),
				})
			}
		case e, ok := <-mc:
			log.Println(e, ok)
			if !ok {
				return
			}
			raw, err := json.Marshal(e)
			if err != nil {
				log.Println(err)
			}
			log.Println(string(raw))
			fmt.Fprint(rw, "event: merge\n")
			fmt.Fprint(rw, "data: ")
			rw.Write(raw)
			fmt.Fprint(rw, "\n\n")
			flusher.Flush()
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

var tileRegex = regexp.MustCompile("([0-9]+)/([0-9]+)/([-0-9]+)_([-0-9]+).png")

func (m *Map) gridTile(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	tile := tileRegex.FindStringSubmatch(req.URL.Path)
	mapid, err := strconv.Atoi(tile[1])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	z, err := strconv.Atoi(tile[2])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	x, err := strconv.Atoi(tile[3])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	y, err := strconv.Atoi(tile[4])
	if err != nil {
		http.Error(rw, "request parsing error", http.StatusInternalServerError)
		return
	}
	td := m.GetTile(mapid, Coord{X: x, Y: y}, z)

	if td == nil {
		http.Error(rw, "file not found", 404)
		return
	}

	rw.Header().Set("Cache-Control", "private immutable")

	http.ServeFile(rw, req, filepath.Join(m.gridStorage, td.File))
}
