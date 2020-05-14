package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func (m *Map) admin(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	users := []string{}
	prefix := ""
	maps := []MapInfo{}
	defaultHide := false
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		config := tx.Bucket([]byte("config"))
		if config != nil {
			prefix = string(config.Get([]byte("prefix")))
			defaultHide = config.Get([]byte("defaultHide")) != nil
		}
		mapB := tx.Bucket([]byte("maps"))
		if mapB != nil {
			mapB.ForEach(func(k, v []byte) error {
				mi := MapInfo{}
				json.Unmarshal(v, &mi)
				maps = append(maps, mi)
				return nil
			})
		}
		return b.ForEach(func(k, v []byte) error {
			users = append(users, string(k))
			return nil
		})
	})

	m.ExecuteTemplate(rw, "admin/index.tmpl", struct {
		Page        Page
		Session     *Session
		Users       []string
		Prefix      string
		DefaultHide bool
		Maps        []MapInfo
	}{
		Page:        m.getPage(req),
		Session:     s,
		Users:       users,
		Prefix:      prefix,
		DefaultHide: defaultHide,
		Maps:        maps,
	})
}

func (m *Map) adminUser(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()
		username := req.FormValue("user")
		password := req.FormValue("pass")
		auths := req.Form["auths"]
		tempAdmin := false
		m.db.Update(func(tx *bbolt.Tx) error {
			users, err := tx.CreateBucketIfNotExists([]byte("users"))
			if err != nil {
				return err
			}
			if s.Username == "admin" && users.Get([]byte("admin")) == nil {
				tempAdmin = true
			}
			u := User{}
			raw := users.Get([]byte(username))
			if raw != nil {
				json.Unmarshal(raw, &u)
			}
			if password != "" {
				u.Pass, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			}
			u.Auths = auths
			raw, _ = json.Marshal(u)
			users.Put([]byte(username), raw)
			return nil
		})
		if username == s.Username {
			s.Auths = auths
		}
		if tempAdmin {
			m.deleteSession(s)
		}
		http.Redirect(rw, req, "/admin", 302)
		return
	}

	user := req.FormValue("user")
	u := User{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		userRaw := b.Get([]byte(user))
		if userRaw == nil {
			return nil
		}
		return json.Unmarshal(userRaw, &u)
	})

	m.ExecuteTemplate(rw, "admin/user.tmpl", struct {
		Page     Page
		Session  *Session
		User     User
		Username string
	}{
		Page:     m.getPage(req),
		Session:  s,
		User:     u,
		Username: user,
	})
}

func (m *Map) wipe(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	err := m.db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("grids")) != nil {
			err := tx.DeleteBucket([]byte("grids"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("markers")) != nil {
			err := tx.DeleteBucket([]byte("markers"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("tiles")) != nil {
			err := tx.DeleteBucket([]byte("tiles"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("maps")) != nil {
			err := tx.DeleteBucket([]byte("maps"))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	/*for z := 0; z <= 5; z++ {
		os.RemoveAll(fmt.Sprintf("%s/%d", m.gridStorage, z))
	}*/
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setPrefix(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("prefix"), []byte(req.FormValue("prefix")))
	})
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setDefaultHide(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		if req.FormValue("defaultHide") != "" {
			return b.Put([]byte("defaultHide"), []byte(req.FormValue("defaultHide")))
		} else {
			return b.Delete([]byte("defaultHide"))
		}
	})
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setTitle(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("title"), []byte(req.FormValue("title")))
	})
	http.Redirect(rw, req, "/admin/", 302)
}

type zoomproc struct {
	c Coord
	m int
}

func (m *Map) rebuildZooms(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	needProcess := map[zoomproc]struct{}{}
	saveGrid := map[zoomproc]string{}

	noGrids := false
	m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("grids"))
		if b == nil {
			noGrids = true
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			grid := GridData{}
			json.Unmarshal(v, &grid)
			needProcess[zoomproc{grid.Coord.Parent(), grid.Map}] = struct{}{}
			saveGrid[zoomproc{grid.Coord, grid.Map}] = grid.ID
			return nil
		})
		tx.DeleteBucket([]byte("tiles"))
		return nil
	})

	if noGrids {
		return
	}
	for g, id := range saveGrid {
		f := fmt.Sprintf("%s/grids/%s.png", m.gridStorage, id)
		if _, err := os.Stat(f); err != nil {
			continue
		}
		m.SaveTile(g.m, g.c, 0, fmt.Sprintf("grids/%s.png", id), time.Now().UnixNano())
	}
	for z := 1; z <= 5; z++ {
		process := needProcess
		needProcess = map[zoomproc]struct{}{}
		for p := range process {
			m.updateZoomLevel(p.m, p.c, z)
			needProcess[zoomproc{p.c.Parent(), p.m}] = struct{}{}
		}
	}
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) deleteUser(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	username := req.FormValue("user")
	m.db.Update(func(tx *bbolt.Tx) error {
		users, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		u := User{}
		raw := users.Get([]byte(username))
		if raw != nil {
			json.Unmarshal(raw, &u)
		}
		tokens, err := tx.CreateBucketIfNotExists([]byte("tokens"))
		if err != nil {
			return err
		}
		for _, tok := range u.Tokens {
			err = tokens.Delete([]byte(tok))
			if err != nil {
				return err
			}
		}
		err = users.Delete([]byte(username))
		if err != nil {
			return err
		}
		return nil
	})
	if username == s.Username {
		m.deleteSession(s)
	}
	http.Redirect(rw, req, "/admin", 302)
	return
}

var errFound = errors.New("found tile")

func (m *Map) wipeTile(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	xraw := req.FormValue("x")
	x, err := strconv.Atoi(xraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	yraw := req.FormValue("y")
	y, err := strconv.Atoi(yraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	c := Coord{
		X: x,
		Y: y,
	}

	m.db.Update(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		ids := [][]byte{}
		err := grids.ForEach(func(k, v []byte) error {
			g := GridData{}
			err := json.Unmarshal(v, &g)
			if err != nil {
				return err
			}
			if g.Coord == c && g.Map == mapid {
				ids = append(ids, k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			grids.Delete(id)
		}

		return nil
	})

	m.SaveTile(mapid, c, 0, "", -1)
	for z := 1; z <= 5; z++ {
		c = c.Parent()
		m.updateZoomLevel(mapid, c, z)
	}
	rw.WriteHeader(200)
}

func (m *Map) setCoords(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	fxraw := req.FormValue("fx")
	fx, err := strconv.Atoi(fxraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	fyraw := req.FormValue("fy")
	fy, err := strconv.Atoi(fyraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	fc := Coord{
		X: fx,
		Y: fy,
	}

	txraw := req.FormValue("tx")
	tx, err := strconv.Atoi(txraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	tyraw := req.FormValue("ty")
	ty, err := strconv.Atoi(tyraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	tc := Coord{
		X: tx,
		Y: ty,
	}

	diff := Coord{
		X: tc.X - fc.X,
		Y: tc.Y - fc.Y,
	}
	tds := []*TileData{}
	m.db.Update(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		mapZooms := tiles.Bucket([]byte(strconv.Itoa(mapid)))
		if mapZooms == nil {
			return nil
		}
		mapTiles := mapZooms.Bucket([]byte("0"))
		err := grids.ForEach(func(k, v []byte) error {
			g := GridData{}
			err := json.Unmarshal(v, &g)
			if err != nil {
				return err
			}
			if g.Map == mapid {
				g.Coord.X += diff.X
				g.Coord.Y += diff.Y
				raw, _ := json.Marshal(g)
				grids.Put(k, raw)
			}
			return nil
		})
		if err != nil {
			return err
		}
		err = mapTiles.ForEach(func(k, v []byte) error {
			td := &TileData{}
			err := json.Unmarshal(v, &td)
			if err != nil {
				return err
			}
			td.Coord.X += diff.X
			td.Coord.Y += diff.Y
			tds = append(tds, td)
			return nil
		})
		if err != nil {
			return err
		}
		err = tiles.DeleteBucket([]byte(strconv.Itoa(mapid)))
		if err != nil {
			return err
		}
		return nil
	})
	needProcess := map[zoomproc]struct{}{}
	for _, td := range tds {
		m.SaveTile(td.MapID, td.Coord, td.Zoom, td.File, time.Now().UnixNano())
		needProcess[zoomproc{c: Coord{X: td.Coord.X, Y: td.Coord.Y}.Parent(), m: td.MapID}] = struct{}{}
	}
	for z := 1; z <= 5; z++ {
		process := needProcess
		needProcess = map[zoomproc]struct{}{}
		for p := range process {
			m.updateZoomLevel(p.m, p.c, z)
			needProcess[zoomproc{p.c.Parent(), p.m}] = struct{}{}
		}
	}
	rw.WriteHeader(200)
}

func (m *Map) backup(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", "attachment; filename=\"backup.zip\"")

	zw := zip.NewWriter(rw)
	defer zw.Close()

	err := m.db.Update(func(tx *bbolt.Tx) error {
		w, err := zw.Create("grids.db")
		if err != nil {
			return err
		}
		err = tx.Copy(w)
		if err != nil {
			return err
		}

		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		zoom := tiles.Bucket([]byte("0"))
		if zoom == nil {
			return nil
		}
		return zoom.ForEach(func(k, v []byte) error {
			td := TileData{}
			json.Unmarshal(v, &td)
			if td.File == "" {
				return nil
			}
			f, err := os.Open(m.gridStorage + "/" + td.File)
			if err != nil {
				return nil
			}
			defer f.Close()
			w, err := zw.Create(td.File)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, f)
			return err
		})
	})
	if err != nil {
		log.Println(err)
	}

}

type mapData struct {
	Grids   map[string]string
	Markers map[string][]Marker
}

func (m *Map) export(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", "attachment; filename=\"griddata.zip\"")

	zw := zip.NewWriter(rw)
	defer zw.Close()

	err := m.db.Update(func(tx *bbolt.Tx) error {
		maps := map[int]mapData{}
		gridMap := map[string]int{}

		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}

		err := grids.ForEach(func(k, v []byte) error {
			gd := GridData{}
			err := json.Unmarshal(v, &gd)
			if err != nil {
				return err
			}
			md, ok := maps[gd.Map]
			if !ok {
				md = mapData{
					Grids:   map[string]string{},
					Markers: map[string][]Marker{},
				}
				maps[gd.Map] = md
			}
			md.Grids[gd.Coord.Name()] = gd.ID
			gridMap[gd.ID] = gd.Map
			mapb := tiles.Bucket([]byte(strconv.Itoa(gd.Map)))
			if mapb == nil {
				return nil
			}
			zoom := mapb.Bucket([]byte("0"))
			if zoom == nil {
				return nil
			}
			tdraw := zoom.Get([]byte(gd.Coord.Name()))
			if tdraw == nil {
				return nil
			}
			td := TileData{}
			err = json.Unmarshal(tdraw, &td)
			if err != nil {
				return err
			}
			w, err := zw.Create(fmt.Sprintf("%d/%s.png", gd.Map, gd.ID))
			if err != nil {
				return err
			}
			f, err := os.Open(filepath.Join(m.gridStorage, td.File))
			if err != nil {
				return err
			}
			defer f.Close()
			io.Copy(w, f)
			return nil
		})
		if err != nil {
			return err
		}

		err = func() error {
			markersb := tx.Bucket([]byte("markers"))
			if markersb == nil {
				return nil
			}
			markersgrid := markersb.Bucket([]byte("grid"))
			if markersgrid == nil {
				return nil
			}
			return markersgrid.ForEach(func(k, v []byte) error {
				m := Marker{}
				err := json.Unmarshal(v, &m)
				if err != nil {
					return nil
				}
				if _, ok := maps[gridMap[m.GridID]]; ok {
					maps[gridMap[m.GridID]].Markers[m.GridID] = append(maps[gridMap[m.GridID]].Markers[m.GridID], m)
				}
				return nil
			})
		}()
		if err != nil {
			return err
		}

		for mapid, mapdata := range maps {
			w, err := zw.Create(fmt.Sprintf("%d/grids.json", mapid))
			if err != nil {
				return err
			}
			json.NewEncoder(w).Encode(mapdata)
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}

}

func (m *Map) hideMarker(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	err := m.db.Update(func(tx *bbolt.Tx) error {
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
		key := idB.Get([]byte(req.FormValue("id")))
		if key == nil {
			return fmt.Errorf("Could not find key %s", req.FormValue("id"))
		}
		raw := grid.Get(key)
		if raw == nil {
			return fmt.Errorf("Could not find key %s", string(key))
		}
		m := Marker{}
		json.Unmarshal(raw, &m)
		m.Hidden = true
		raw, _ = json.Marshal(m)
		grid.Put(key, raw)
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	return
}

func (m *Map) merge(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(1024 * 1024 * 500)
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	mergef, hdr, err := req.FormFile("merge")
	if err != nil {
		log.Println(err)
		http.Error(rw, "request error", http.StatusBadRequest)
		return
	}
	zr, err := zip.NewReader(mergef, hdr.Size)
	if err != nil {
		log.Println(err)
		http.Error(rw, "request error", http.StatusBadRequest)
		return
	}

	ops := []struct {
		mapid int
		x, y  int
		f     string
	}{}
	newTiles := map[string]struct{}{}

	err = m.db.Update(func(tx *bbolt.Tx) error {
		grids, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
		if err != nil {
			return err
		}
		mb, err := tx.CreateBucketIfNotExists([]byte("markers"))
		if err != nil {
			return err
		}
		mgrid, err := mb.CreateBucketIfNotExists([]byte("grid"))
		if err != nil {
			return err
		}
		idB, err := mb.CreateBucketIfNotExists([]byte("id"))
		if err != nil {
			return err
		}
		configb, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		for _, fhdr := range zr.File {
			if strings.HasSuffix(fhdr.Name, ".json") {
				f, err := fhdr.Open()
				if err != nil {
					return err
				}
				md := mapData{}
				err = json.NewDecoder(f).Decode(&md)
				if err != nil {
					return err
				}

				for _, ms := range md.Markers {
					for _, mraw := range ms {
						key := []byte(fmt.Sprintf("%s_%d_%d", mraw.GridID, mraw.Position.X, mraw.Position.Y))
						if mgrid.Get(key) != nil {
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
								X: mraw.Position.X,
								Y: mraw.Position.Y,
							},
							Image: mraw.Image,
						}
						raw, _ := json.Marshal(m)
						mgrid.Put(key, raw)
						idB.Put(idKey, key)
					}
				}

				mapB, err := tx.CreateBucketIfNotExists([]byte("maps"))
				if err != nil {
					return err
				}

				newGrids := map[Coord]string{}
				maps := map[int]struct{ X, Y int }{}
				for k, v := range md.Grids {
					c := Coord{}
					_, err := fmt.Sscanf(k, "%d_%d", &c.X, &c.Y)
					if err != nil {
						return err
					}
					newGrids[c] = v
					gridRaw := grids.Get([]byte(v))
					if gridRaw != nil {
						gd := GridData{}
						json.Unmarshal(gridRaw, &gd)
						maps[gd.Map] = struct{ X, Y int }{gd.Coord.X - c.X, gd.Coord.Y - c.Y}
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
					for c, grid := range newGrids {
						cur := GridData{}
						cur.ID = grid
						cur.Map = int(seq)
						cur.Coord = c

						raw, err := json.Marshal(cur)
						if err != nil {
							return err
						}
						grids.Put([]byte(grid), raw)
					}
					continue
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

				for c, grid := range newGrids {
					cur := GridData{}
					if curRaw := grids.Get([]byte(grid)); curRaw != nil {
						continue
					}

					cur.ID = grid
					cur.Map = mapid
					cur.Coord.X = c.X + offset.X
					cur.Coord.Y = c.Y + offset.Y
					raw, err := json.Marshal(cur)
					if err != nil {
						return err
					}
					grids.Put([]byte(grid), raw)
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

			} else if strings.HasSuffix(fhdr.Name, ".png") {
				os.MkdirAll(filepath.Join(m.gridStorage, "grids"), 0777)
				f, err := os.Create(filepath.Join(m.gridStorage, "grids", filepath.Base(fhdr.Name)))
				if err != nil {
					return err
				}
				r, err := fhdr.Open()
				if err != nil {
					f.Close()
					return err
				}
				io.Copy(f, r)
				r.Close()
				f.Close()
				newTiles[strings.TrimSuffix(filepath.Base(fhdr.Name), ".png")] = struct{}{}
			}
		}

		for gid := range newTiles {
			gridRaw := grids.Get([]byte(gid))
			if gridRaw != nil {
				gd := GridData{}
				json.Unmarshal(gridRaw, &gd)
				ops = append(ops, struct {
					mapid int
					x     int
					y     int
					f     string
				}{
					mapid: gd.Map,
					x:     gd.Coord.X,
					y:     gd.Coord.Y,
					f:     filepath.Join("grids", gid+".png"),
				})
			}
		}
		return nil
	})

	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	for _, op := range ops {
		m.SaveTile(op.mapid, Coord{X: op.x, Y: op.y}, 0, op.f, time.Now().UnixNano())
	}
	m.rebuildZooms(rw, req)
}

func (m *Map) adminICMap(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "map parse failed", http.StatusBadRequest)
		return
	}

	action := req.FormValue("action")

	m.db.Update(func(tx *bbolt.Tx) error {
		maps, err := tx.CreateBucketIfNotExists([]byte("maps"))
		if err != nil {
			return err
		}
		rawmap := maps.Get([]byte(strconv.Itoa(mapid)))
		mapinfo := MapInfo{}
		if rawmap != nil {
			json.Unmarshal(rawmap, &mapinfo)
		}
		switch action {
		case "toggle-hidden":
			mapinfo.Hidden = !mapinfo.Hidden
			m.ExecuteTemplate(rw, "admin/index.tmpl:toggle-hidden", mapinfo)
		}
		rawmap, err = json.Marshal(mapinfo)
		if err != nil {
			return err
		}
		return maps.Put([]byte(strconv.Itoa(mapid)), rawmap)
	})
}

func (m *Map) adminMap(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "map parse failed", http.StatusBadRequest)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()

		name := req.FormValue("name")
		hidden := !(req.FormValue("hidden") == "")
		priority := !(req.FormValue("priority") == "")

		m.db.Update(func(tx *bbolt.Tx) error {
			maps, err := tx.CreateBucketIfNotExists([]byte("maps"))
			if err != nil {
				return err
			}
			rawmap := maps.Get([]byte(strconv.Itoa(mapid)))
			mapinfo := MapInfo{}
			if rawmap != nil {
				json.Unmarshal(rawmap, &mapinfo)
			}
			mapinfo.Name = name
			mapinfo.Hidden = hidden
			mapinfo.Priority = priority
			rawmap, err = json.Marshal(mapinfo)
			if err != nil {
				return err
			}
			return maps.Put([]byte(strconv.Itoa(mapid)), rawmap)
		})

		http.Redirect(rw, req, "/admin", 302)
		return
	}
	mi := MapInfo{}
	m.db.View(func(tx *bbolt.Tx) error {
		mapB := tx.Bucket([]byte("maps"))
		if mapB == nil {
			return nil
		}
		mraw := mapB.Get([]byte(strconv.Itoa(mapid)))
		return json.Unmarshal(mraw, &mi)
	})

	m.ExecuteTemplate(rw, "admin/map.tmpl", struct {
		Page    Page
		Session *Session
		MapInfo MapInfo
	}{
		Page:    m.getPage(req),
		Session: s,
		MapInfo: mi,
	})
}
