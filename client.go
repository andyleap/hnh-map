package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
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
	err := json.NewDecoder(req.Body).Decode(&craw)
	if err != nil {
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
	err := json.NewDecoder(req.Body).Decode(&markers)
	log.Println(markers)
	if err != nil {
		return
	}
	user, ok := req.Context().Value(UserInfo).(string)
	if !ok {
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		markersB, err := tx.CreateBucketIfNotExists([]byte("markers"))
		if err != nil {
			return err
		}
		grids, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		ms := []Marker{}
		for _, mraw := range markers {
			gridRaw := grids.Get([]byte(strconv.Itoa(mraw.GridID)))
			grid := GridData{}
			if gridRaw == nil {
				continue
			}
			err := json.Unmarshal(gridRaw, &grid)
			if err != nil {
				return err
			}
			if mraw.Image == "" {
				mraw.Image = "gfx/terobjs/mm/custom"
			}
			m := Marker{
				Name: mraw.Name,
				ID:   1,
				Position: Position{
					X: mraw.X + grid.Coord.X*100,
					Y: mraw.Y + grid.Coord.Y*100,
				},
				Image: mraw.Image,
			}
			ms = append(ms, m)
		}
		raw, _ := json.Marshal(ms)
		markersB.Put([]byte(user), raw)
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
	parts := strings.SplitN(req.Header.Get("Content-Type"), "=", 2)
	req.Header.Set("Content-Type", parts[0]+"=\""+parts[1]+"\"")

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

	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		curRaw := b.Get([]byte(id))
		cur := GridData{}
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

		if time.Now().After(cur.NextUpdate) {
			os.MkdirAll(fmt.Sprintf("%s/0", m.gridStorage), 0600)
			f, err := os.Create(fmt.Sprintf("%s/0/%s", m.gridStorage, cur.Coord.Name()))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, file)
			if err != nil {
				f.Close()
				return err
			}
			f.Close()

			m.updateZooms(cur.Coord)
			cur.NextUpdate = time.Now().Add(time.Minute * 30)
		}

		raw, err := json.Marshal(cur)
		if err != nil {
			return err
		}
		b.Put([]byte(id), raw)

		return nil
	})
}

func (m *Map) updateZooms(c Coord) {
	factor := 1
	for z := 1; z <= 5; z++ {
		factor = factor * 2
		sX := (c.X % factor)
		if sX < 0 {
			sX = -sX
		}
		c.X = c.X - sX
		sY := (c.Y % factor)
		if sY < 0 {
			sY = -sY
		}
		c.Y = c.Y - sY
		m.updateZoomLevel(c, z, factor/2)
	}
}

func (m *Map) updateZoomLevel(c Coord, z int, factor int) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)
	for x := 0; x <= 1; x++ {
		for y := 0; y <= 1; y++ {
			subC := c
			subC.X += x * factor
			subC.Y += y * factor
			subf, err := os.Open(fmt.Sprintf("%s/%d/%s", m.gridStorage, z-1, subC.Name()))
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
	if err != nil {
		return
	}
	defer f.Close()
	png.Encode(f, img)
}
