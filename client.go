package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"

	"go.etcd.io/bbolt"
)

var clientPath = regexp.MustCompile("client/([^/]+)/(.*)")

var UserInfo struct{}

const VERSION = "4"

func (m *Map) client(rw http.ResponseWriter, req *http.Request) {
	matches := clientPath.FindStringSubmatch(req.URL.Path)
	if matches == nil {
		http.Error(rw, "Client token not found", http.StatusBadRequest)
		return
	}
	auth := false
	user := ""
	m.db.View(func(tx *bbolt.Tx) error {
		tb := tx.Bucket([]byte("tokens"))
		if tb == nil {
			return nil
		}
		userName := tb.Get([]byte(matches[1]))
		if userName == nil {
			return nil
		}
		ub := tx.Bucket([]byte("users"))
		if ub == nil {
			return nil
		}
		userRaw := ub.Get(userName)
		if userRaw == nil {
			return nil
		}
		u := User{}
		json.Unmarshal(userRaw, &u)
		if u.Auths.Has(AUTH_UPLOAD) {
			user = string(userName)
			auth = true
		}
		return nil
	})
	if !auth {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx := context.WithValue(req.Context(), UserInfo, user)
	req = req.WithContext(ctx)

	switch matches[2] {
	case "locate":
		m.locate(rw, req)
	case "gridUpdate":
		m.gridUpdate(rw, req)
	case "gridUpload":
		m.gridUpload(rw, req)
	case "positionUpdate":
		m.updatePositions(rw, req)
	case "markerUpdate":
		m.uploadMarkers(rw, req)
	/*case "mapData":
	m.mapdataIndex(rw, req)*/
	case "":
		http.Redirect(rw, req, "/map/", 302)
	case "checkVersion":
		if req.FormValue("version") == VERSION {
			rw.WriteHeader(200)
		} else {
			rw.WriteHeader(http.StatusBadRequest)
		}
	default:
		rw.WriteHeader(http.StatusNotFound)
	}
}

func (m *Map) updatePositions(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	craws := map[string]struct {
		Name   string
		GridID string
		Coords struct {
			X, Y int
		}
		Type string
	}{}
	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println("Error reading position update json: ", err)
		return
	}
	err = json.Unmarshal(buf, &craws)
	if err != nil {
		log.Println("Error decoding position update json: ", err)
		log.Println("Original json: ", string(buf))
		return
	}
	m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		m.chmu.Lock()
		defer m.chmu.Unlock()
		for id, craw := range craws {
			grid := grids.Get([]byte(craw.GridID))
			if grid == nil {
				return nil
			}
			gd := GridData{}
			json.Unmarshal(grid, &gd)
			idnum, _ := strconv.Atoi(id)
			c := Character{
				Name: craw.Name,
				ID:   idnum,
				Map:  gd.Map,
				Position: Position{
					X: craw.Coords.X + (gd.Coord.X * 100),
					Y: craw.Coords.Y + (gd.Coord.Y * 100),
				},
				Type:    craw.Type,
				updated: time.Now(),
			}
			old, ok := m.characters[id]
			if !ok {
				m.characters[id] = c
			} else {
				if old.Type == "player" {
					if c.Type == "player" {
						m.characters[id] = c
					} else {
						old.Position = c.Position
						m.characters[id] = old
					}
				} else if old.Type != "unknown" {
					if c.Type != "unknown" {
						m.characters[id] = c
					} else {
						old.Position = c.Position
						m.characters[id] = old
					}
				} else {
					m.characters[id] = c
				}
			}
		}
		return nil
	})

}

func (m *Map) uploadMarkers(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	markers := []struct {
		Name   string
		GridID string
		X, Y   int
		Image  string
		Type   string
		Color  string
	}{}
	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println("Error reading marker json: ", err)
		return
	}
	err = json.Unmarshal(buf, &markers)
	if err != nil {
		log.Println("Error decoding marker json: ", err)
		log.Println("Original json: ", string(buf))
		return
	}
	err = m.db.Update(func(tx *bbolt.Tx) error {
		mb, err := tx.CreateBucketIfNotExists([]byte("markers"))
		if err != nil {
			return err
		}
		grid, err := mb.CreateBucketIfNotExists([]byte("grid"))
		if err != nil {
			return err
		}
		idB, err := mb.CreateBucketIfNotExists([]byte("id"))
		if err != nil {
			return err
		}

		for _, mraw := range markers {
			key := []byte(fmt.Sprintf("%s_%d_%d", mraw.GridID, mraw.X, mraw.Y))
			if grid.Get(key) != nil {
				continue
			}
			if mraw.Image == "" {
				mraw.Image = "gfx/terobjs/mm/custom"
			}
			id, err := idB.NextSequence()
			if err != nil {
				return err
			}
			idKey := []byte(strconv.Itoa(int(id)))
			m := Marker{
				Name:   mraw.Name,
				ID:     int(id),
				GridID: mraw.GridID,
				Position: Position{
					X: mraw.X,
					Y: mraw.Y,
				},
				Image: mraw.Image,
			}
			raw, _ := json.Marshal(m)
			grid.Put(key, raw)
			idB.Put(idKey, key)
		}
		return nil
	})
	if err != nil {
		log.Println("Error update db: ", err)
		return
	}
}

func (m *Map) locate(rw http.ResponseWriter, req *http.Request) {
	grid := req.FormValue("gridID")
	err := m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		curRaw := grids.Get([]byte(grid))
		cur := GridData{}
		if curRaw == nil {
			return fmt.Errorf("grid not found")
		}
		err := json.Unmarshal(curRaw, &cur)
		if err != nil {
			return err
		}
		fmt.Fprintf(rw, "%d;%d;%d", cur.Map, cur.Coord.X, cur.Coord.Y)
		return nil
	})
	if err != nil {
		rw.WriteHeader(404)
	}
}

type GridUpdate struct {
	Grids [][]string `json:"grids"`
}

type GridRequest struct {
	GridRequests []string `json:"gridRequests"`
	Map          int      `json:"map"`
	Coords       Coord    `json:"coords"`
}

func (m *Map) gridUpdate(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	dec := json.NewDecoder(req.Body)
	grup := GridUpdate{}
	err := dec.Decode(&grup)
	if err != nil {
		log.Println("Error decoding grid request json: ", err)
		http.Error(rw, "Error decoding request", http.StatusBadRequest)
		return
	}
	log.Println(grup)

	ops := []struct {
		mapid int
		x, y  int
		f     string
	}{}

	greq := GridRequest{}

	err = m.db.Update(func(tx *bbolt.Tx) error {
		grids, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
		if err != nil {
			return err
		}

		mapB, err := tx.CreateBucketIfNotExists([]byte("maps"))
		if err != nil {
			return err
		}

		configb, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}

		maps := map[int]struct{ X, Y int }{}
		for x, row := range grup.Grids {
			for y, grid := range row {
				gridRaw := grids.Get([]byte(grid))
				if gridRaw != nil {
					gd := GridData{}
					json.Unmarshal(gridRaw, &gd)
					maps[gd.Map] = struct{ X, Y int }{gd.Coord.X - x, gd.Coord.Y - y}
				}
			}
		}

		if len(maps) == 0 {
			seq, err := mapB.NextSequence()
			if err != nil {
				return err
			}
			mi := MapInfo{
				ID:     int(seq),
				Name:   strconv.Itoa(int(seq)),
				Hidden: configb.Get([]byte("defaultHide")) != nil,
			}
			raw, _ := json.Marshal(mi)
			err = mapB.Put([]byte(strconv.Itoa(int(seq))), raw)
			if err != nil {
				return err
			}
			log.Println("Client made mapid ", seq)
			for x, row := range grup.Grids {
				for y, grid := range row {

					cur := GridData{}
					cur.ID = grid
					cur.Map = int(seq)
					cur.Coord.X = x - 1
					cur.Coord.Y = y - 1

					raw, err := json.Marshal(cur)
					if err != nil {
						return err
					}
					grids.Put([]byte(grid), raw)
					greq.GridRequests = append(greq.GridRequests, grid)
				}
			}
			greq.Coords = Coord{0, 0}
			return nil
		}

		mapid := -1
		offset := struct{ X, Y int }{}
		for id, off := range maps {
			mi := MapInfo{}
			mraw := mapB.Get([]byte(strconv.Itoa(id)))
			if mraw != nil {
				json.Unmarshal(mraw, &mi)
			}
			if mi.Priority {
				mapid = id
				offset = off
				break
			}
			if id < mapid || mapid == -1 {
				mapid = id
				offset = off
			}
		}

		log.Println("Client in mapid ", mapid)

		for x, row := range grup.Grids {
			for y, grid := range row {
				cur := GridData{}
				if curRaw := grids.Get([]byte(grid)); curRaw != nil {
					json.Unmarshal(curRaw, &cur)
					if time.Now().After(cur.NextUpdate) {
						greq.GridRequests = append(greq.GridRequests, grid)
					}
					continue
				}

				cur.ID = grid
				cur.Map = mapid
				cur.Coord.X = x + offset.X
				cur.Coord.Y = y + offset.Y
				raw, err := json.Marshal(cur)
				if err != nil {
					return err
				}
				grids.Put([]byte(grid), raw)
				greq.GridRequests = append(greq.GridRequests, grid)
			}
		}
		if curRaw := grids.Get([]byte(grup.Grids[1][1])); curRaw != nil {
			cur := GridData{}
			json.Unmarshal(curRaw, &cur)
			greq.Map = cur.Map
			greq.Coords = cur.Coord
		}
		if len(maps) > 1 {
			grids.ForEach(func(k, v []byte) error {
				gd := GridData{}
				json.Unmarshal(v, &gd)
				if gd.Map == mapid {
					return nil
				}
				if merge, ok := maps[gd.Map]; ok {
					var td *TileData
					mapb, err := tiles.CreateBucketIfNotExists([]byte(strconv.Itoa(gd.Map)))
					if err != nil {
						return err
					}
					zoom, err := mapb.CreateBucketIfNotExists([]byte(strconv.Itoa(0)))
					if err != nil {
						return err
					}
					tileraw := zoom.Get([]byte(gd.Coord.Name()))
					if tileraw != nil {
						json.Unmarshal(tileraw, &td)
					}

					gd.Map = mapid
					gd.Coord.X += offset.X - merge.X
					gd.Coord.Y += offset.Y - merge.Y
					raw, _ := json.Marshal(gd)
					if td != nil {
						ops = append(ops, struct {
							mapid int
							x     int
							y     int
							f     string
						}{
							mapid: mapid,
							x:     gd.Coord.X,
							y:     gd.Coord.Y,
							f:     td.File,
						})
					}
					grids.Put(k, raw)
				}
				return nil
			})
		}
		for mergeid, merge := range maps {
			if mapid == mergeid {
				continue
			}
			mapB.Delete([]byte(strconv.Itoa(mergeid)))
			log.Println("Reporting merge", mergeid, mapid)
			m.reportMerge(mergeid, mapid, Coord{X: offset.X - merge.X, Y: offset.Y - merge.Y})
		}
		return nil
	})
	if err != nil {
		log.Println(err)
		return
	}
	needProcess := map[zoomproc]struct{}{}
	for _, op := range ops {
		m.SaveTile(op.mapid, Coord{X: op.x, Y: op.y}, 0, op.f, time.Now().UnixNano())
		needProcess[zoomproc{c: Coord{X: op.x, Y: op.y}.Parent(), m: op.mapid}] = struct{}{}
	}
	for z := 1; z <= 5; z++ {
		process := needProcess
		needProcess = map[zoomproc]struct{}{}
		for p := range process {
			m.updateZoomLevel(p.m, p.c, z)
			needProcess[zoomproc{p.c.Parent(), p.m}] = struct{}{}
		}
	}
	log.Println(greq)
	json.NewEncoder(rw).Encode(greq)
}

/*
func (m *Map) mapdataIndex(rw http.ResponseWriter, req *http.Request) {
	err := m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return fmt.Errorf("grid not found")
		}
		return grids.ForEach(func(k, v []byte) error {
			cur := GridData{}
			err := json.Unmarshal(v, &cur)
			if err != nil {
				return err
			}
			fmt.Fprintf(rw, "%s,%d,%d,%d\n", cur.ID, cur.Map, cur.Coord.X, cur.Coord.Y)
			return nil
		})
	})
	if err != nil {
		rw.WriteHeader(404)
	}
}
*/

type ExtraData struct {
	Season int
}

func (m *Map) gridUpload(rw http.ResponseWriter, req *http.Request) {
	if strings.Count(req.Header.Get("Content-Type"), "=") >= 2 && strings.Count(req.Header.Get("Content-Type"), "\"") == 0 {
		parts := strings.SplitN(req.Header.Get("Content-Type"), "=", 2)
		req.Header.Set("Content-Type", parts[0]+"=\""+parts[1]+"\"")
	}

	err := req.ParseMultipartForm(100000000)
	if err != nil {
		log.Println(err)
		return
	}

	id := req.FormValue("id")

	extraData := req.FormValue("extraData")
	if extraData != "" {
		ed := ExtraData{}
		json.Unmarshal([]byte(extraData), &ed)
		if ed.Season == 3 {
			needTile := false
			m.db.Update(func(tx *bbolt.Tx) error {
				b, err := tx.CreateBucketIfNotExists([]byte("grids"))
				if err != nil {
					return err
				}
				curRaw := b.Get([]byte(id))
				if curRaw == nil {
					return fmt.Errorf("Unknown grid id: %s", id)
				}
				cur := GridData{}
				err = json.Unmarshal(curRaw, &cur)
				if err != nil {
					return err
				}

				tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
				if err != nil {
					return err
				}
				maps, err := tiles.CreateBucketIfNotExists([]byte(strconv.Itoa(cur.Map)))
				if err != nil {
					return err
				}
				zooms, err := maps.CreateBucketIfNotExists([]byte("0"))
				if err != nil {
					return err
				}

				tdRaw := zooms.Get([]byte(cur.Coord.Name()))
				if tdRaw == nil {
					needTile = true
					return nil
				}
				td := TileData{}
				err = json.Unmarshal(tdRaw, &td)
				if err != nil {
					return err
				}
				if td.File == "" {
					needTile = true
					return nil
				}

				if time.Now().After(cur.NextUpdate) {
					cur.NextUpdate = time.Now().Add(time.Minute * 30)
				}

				raw, err := json.Marshal(cur)
				if err != nil {
					return err
				}
				b.Put([]byte(id), raw)

				return nil
			})
			if !needTile {
				log.Println("ignoring tile upload: winter")
				return
			} else {
				log.Println("Missing tile, using winter version")
			}
		}
	}

	file, _, err := req.FormFile("file")
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("map tile for ", id)

	updateTile := false
	cur := GridData{}

	mapid := 0

	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		curRaw := b.Get([]byte(id))
		if curRaw == nil {
			return fmt.Errorf("Unknown grid id: %s", id)
		}
		err = json.Unmarshal(curRaw, &cur)
		if err != nil {
			return err
		}

		updateTile = time.Now().After(cur.NextUpdate)
		mapid = cur.Map

		if updateTile {
			cur.NextUpdate = time.Now().Add(time.Minute * 30)
		}

		raw, err := json.Marshal(cur)
		if err != nil {
			return err
		}
		b.Put([]byte(id), raw)

		return nil
	})

	if updateTile {
		os.MkdirAll(fmt.Sprintf("%s/grids", m.gridStorage), 0600)
		f, err := os.Create(fmt.Sprintf("%s/grids/%s.png", m.gridStorage, cur.ID))
		if err != nil {
			return
		}
		_, err = io.Copy(f, file)
		if err != nil {
			f.Close()
			return
		}
		f.Close()

		m.SaveTile(mapid, cur.Coord, 0, fmt.Sprintf("grids/%s.png", cur.ID), time.Now().UnixNano())

		c := cur.Coord
		for z := 1; z <= 5; z++ {
			c = c.Parent()
			m.updateZoomLevel(mapid, c, z)
		}
	}
}

func (m *Map) updateZoomLevel(mapid int, c Coord, z int) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)
	for x := 0; x <= 1; x++ {
		for y := 0; y <= 1; y++ {
			subC := c
			subC.X *= 2
			subC.Y *= 2
			subC.X += x
			subC.Y += y
			td := m.GetTile(mapid, subC, z-1)
			if td == nil || td.File == "" {
				continue
			}
			subf, err := os.Open(filepath.Join(m.gridStorage, td.File))
			if err != nil {
				continue
			}
			subimg, _, err := image.Decode(subf)
			subf.Close()
			if err != nil {
				continue
			}
			draw.BiLinear.Scale(img, image.Rect(50*x, 50*y, 50*x+50, 50*y+50), subimg, subimg.Bounds(), draw.Src, nil)
		}
	}
	os.MkdirAll(fmt.Sprintf("%s/%d/%d", m.gridStorage, mapid, z), 0600)
	f, err := os.Create(fmt.Sprintf("%s/%d/%d/%s.png", m.gridStorage, mapid, z, c.Name()))
	m.SaveTile(mapid, c, z, fmt.Sprintf("%d/%d/%s.png", mapid, z, c.Name()), time.Now().UnixNano())
	if err != nil {
		return
	}
	defer func() {
		f.Close()
	}()
	png.Encode(f, img)
}
