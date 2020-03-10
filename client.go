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
	case "api/v1/locate":
		m.locate(rw, req)
	case "api/v2/updateGrid":
		m.uploadMinimap(rw, req)
	case "api/v2/updateCharacter":
		m.updateChar(rw, req)
	case "api/v1/uploadMarkers":
		m.uploadMarkers(rw, req)
	case "grids/mapdata_index":
		m.mapdataIndex(rw, req)
	case "":
		http.Redirect(rw, req, "/map/", 302)
	default:
		rw.WriteHeader(http.StatusNotFound)
	}
}

func (m *Map) updateChar(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	craw := struct {
		Name string
		ID   int
		X, Y int
		Type string
	}{}
	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println("Error reading char update json: ", err)
		return
	}
	err = json.Unmarshal(buf, &craw)
	if err != nil {
		log.Println("Error decoding char update json: ", err)
		log.Println("Original json: ", string(buf))
		return
	}
	c := Character{
		Name: craw.Name,
		ID:   craw.ID,
		Position: Position{
			X: craw.X,
			Y: craw.Y,
		},
		Type:    craw.Type,
		updated: time.Now(),
	}
	m.chmu.Lock()
	defer m.chmu.Unlock()
	m.characters[c.Name] = c
}

func (m *Map) uploadMarkers(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	markers := []struct {
		Name   string
		GridID int
		X, Y   int
		Image  string
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
	m.db.Update(func(tx *bbolt.Tx) error {
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
			key := []byte(fmt.Sprintf("%d_%d_%d", mraw.GridID, mraw.X, mraw.Y))
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
}

func (m *Map) locate(rw http.ResponseWriter, req *http.Request) {
	grid := req.FormValue("gridId")
	setZero := false
	err := m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			setZero = true
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
		fmt.Fprintf(rw, "%d;%d", cur.Coord.X, cur.Coord.Y)
		return nil
	})
	if setZero {
		err = m.db.Update(func(tx *bbolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte("grids"))
			if err != nil {
				return err
			}
			cur := GridData{}
			cur.ID = grid
			cur.Coord.X = 0
			cur.Coord.Y = 0

			raw, err := json.Marshal(cur)
			if err != nil {
				return err
			}
			b.Put([]byte(grid), raw)
			fmt.Fprintf(rw, "%d;%d", cur.Coord.X, cur.Coord.Y)
			return nil
		})
	}
	if err != nil {
		rw.WriteHeader(404)
	}
}

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
			fmt.Fprintf(rw, "%s,%d,%d\n", cur.ID, cur.Coord.X, cur.Coord.Y)
			return nil
		})
	})
	if err != nil {
		rw.WriteHeader(404)
	}
}

func (m *Map) uploadMinimap(rw http.ResponseWriter, req *http.Request) {
	if strings.Count(req.Header.Get("Content-Type"), "=") >= 2 && strings.Count(req.Header.Get("Content-Type"), "\"") == 0 {
		parts := strings.SplitN(req.Header.Get("Content-Type"), "=", 2)
		req.Header.Set("Content-Type", parts[0]+"=\""+parts[1]+"\"")
	}

	err := req.ParseMultipartForm(100000000)
	if err != nil {
		log.Panic(err)
	}
	file, _, err := req.FormFile("file")
	if err != nil {
		log.Panic(err)
	}
	id := req.FormValue("id")
	xraw := req.FormValue("x")
	yraw := req.FormValue("y")

	x, err := strconv.Atoi(xraw)
	if err != nil {
		log.Println(err)
		return
	}
	y, err := strconv.Atoi(yraw)
	if err != nil {
		log.Println(err)
		return
	}

	updateTile := false
	cur := GridData{}

	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		curRaw := b.Get([]byte(id))
		if curRaw != nil {
			err := json.Unmarshal(curRaw, &cur)
			if err != nil {
				return err
			}
			if cur.Coord.X != x || cur.Coord.Y != y {
				return fmt.Errorf("invalid coords")
			}
		} else {
			cur.ID = id
			cur.Coord.X = x
			cur.Coord.Y = y
		}

		updateTile = time.Now().After(cur.NextUpdate)

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
		os.MkdirAll(fmt.Sprintf("%s/0", m.gridStorage), 0600)
		f, err := os.Create(fmt.Sprintf("%s/0/%s", m.gridStorage, cur.ID))
		if err != nil {
			return
		}
		_, err = io.Copy(f, file)
		if err != nil {
			f.Close()
			return
		}
		f.Close()

		m.SaveTile(cur.Coord, 0, fmt.Sprintf("0/%s", cur.ID), time.Now().UnixNano())

		c := cur.Coord
		for z := 1; z <= 5; z++ {
			c = c.Parent()
			m.updateZoomLevel(c, z)
		}
	}
}

func (m *Map) updateZoomLevel(c Coord, z int) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)
	for x := 0; x <= 1; x++ {
		for y := 0; y <= 1; y++ {
			subC := c
			subC.X *= 2
			subC.Y *= 2
			subC.X += x
			subC.Y += y
			td := m.GetTile(subC, z-1)
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
			draw.BiLinear.Scale(img, image.Rect(50*x, 50*y, 50*x+50, 50*y+50), subimg, subimg.Bounds(), draw.Over, nil)
		}
	}
	os.MkdirAll(fmt.Sprintf("%s/%d", m.gridStorage, z), 0600)
	f, err := os.Create(fmt.Sprintf("%s/%d/%s", m.gridStorage, z, c.Name()))
	m.SaveTile(c, z, fmt.Sprintf("%d/%s", z, c.Name()), time.Now().UnixNano())
	if err != nil {
		return
	}
	defer func() {
		f.Close()
	}()
	png.Encode(f, img)
}
