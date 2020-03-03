package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"

	"go.etcd.io/bbolt"
)

type Map struct {
	gridStorage string
	db          *bbolt.DB
}

var (
	gridStorage = flag.String("grids", "grids", "directory to store grids in")
)

func main() {
	flag.Parse()

	db, err := bbolt.Open(*gridStorage+"/grids.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	m := Map{
		gridStorage: *gridStorage,
		db:          db,
	}

	http.HandleFunc("/api/v1/locate", m.locate)
	http.HandleFunc("/api/v2/updateGrid", m.uploadMinimap)
	http.HandleFunc("/api/v1/setZeroGrid", m.setZero)
	http.HandleFunc("/api/v2/updateCharacter", m.updateChar)
	http.HandleFunc("/api/v1/characters", m.getChars)
	http.HandleFunc("/api/v1/markers", m.getMarkers)

	http.Handle("/grids/", http.StripPrefix("/grids", http.FileServer(http.Dir(m.gridStorage))))
	http.Handle("/", http.FileServer(http.Dir("frontend")))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Character struct {
	Name     string `json:"name"`
	ID       int    `json:"id"`
	Position Coord  `json:"position"`
	Type     string `json:"type"`
}

type GridData struct {
	ID         string
	Coord      Coord
	NextUpdate time.Time
}

type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (c Coord) Name() string {
	return fmt.Sprintf("%d_%d.png", c.X, c.Y)
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
		Position: Coord{
			X: craw.X,
			Y: craw.Y,
		},
		Type: craw.Type,
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		chars, err := tx.CreateBucketIfNotExists([]byte("chars"))
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(c)
		chars.Put([]byte(strconv.Itoa(c.ID)), raw)
		return nil
	})
}

func (m *Map) getChars(rw http.ResponseWriter, req *http.Request) {
	chars := []Character{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("chars"))
		if b == nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			c := Character{}
			json.Unmarshal(v, &c)
			chars = append(chars, c)
			return nil
		})
		return nil
	})
	json.NewEncoder(rw).Encode(chars)
}

func (m *Map) getMarkers(rw http.ResponseWriter, req *http.Request) {
	chars := []Character{}
	json.NewEncoder(rw).Encode(chars)
}

func (m *Map) setZero(rw http.ResponseWriter, req *http.Request) {
	grid := req.FormValue("gridId")
	m.db.Update(func(tx *bbolt.Tx) error {
		tx.DeleteBucket([]byte("grids"))
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
		return nil
	})
	os.RemoveAll(m.gridStorage)
}

func (m *Map) locate(rw http.ResponseWriter, req *http.Request) {
	grid := req.FormValue("gridId")
	err := m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return fmt.Errorf("grid not found")
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
