package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"go.etcd.io/bbolt"
)

type Map struct {
	gridStorage string
	db          *bbolt.DB

	characters map[string]Character
	chmu       sync.RWMutex
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

		characters: map[string]Character{},
	}

	go m.cleanChars()

	// Mapping client endpoints
	http.HandleFunc("/api/v1/locate", m.locate)
	http.HandleFunc("/api/v2/updateGrid", m.uploadMinimap)
	http.HandleFunc("/api/v2/updateCharacter", m.updateChar)
	http.HandleFunc("/api/v1/uploadMarkers", m.uploadMarkers)
	http.HandleFunc("/grids/mapdata_index", m.mapdataIndex)

	// Map frontend endpoints
	http.HandleFunc("/api/v1/characters", m.getChars)
	http.HandleFunc("/api/v1/markers", m.getMarkers)

	// Admin endpoints
	http.HandleFunc("/api/admin/setZeroGrid", m.setZero)
	http.HandleFunc("/api/admin/setUser", m.setUser)

	http.Handle("/grids/", http.StripPrefix("/grids", http.FileServer(http.Dir(m.gridStorage))))
	http.Handle("/", http.FileServer(http.Dir("frontend")))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Character struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	Position Position `json:"position"`
	Type     string   `json:"type"`
	updated  time.Time
}

type Marker struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	Position Position `json:"position"`
	Image    string   `json:"image"`
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

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (c Coord) Name() string {
	return fmt.Sprintf("%d_%d.png", c.X, c.Y)
}

type Auths []string

func (a Auths) Has(auth string) bool {
	for _, v := range a {
		if v == auth {
			return true
		}
	}
	return false
}

const (
	AUTH_ADMIN  = "admin"
	AUTH_MAP    = "map"
	AUTH_UPLOAD = "upload"
)

type User struct {
	Pass  []byte
	Auths Auths
}

func (m *Map) getAuth(user, pass string) Auths {
	auth := Auths{}
	m.db.View(func(tx *bbolt.Tx) error {
		users := tx.Bucket([]byte("users"))
		if users == nil {
			if user == "admin" && pass == "admin" {
				auth = Auths{"admin"}
			}
			return nil
		}
		raw := users.Get([]byte(user))
		if raw != nil {
			u := User{}
			json.Unmarshal(raw, &u)
			if bcrypt.CompareHashAndPassword(u.Pass, []byte(pass)) != nil {
				return nil
			}
			auth = u.Auths
		}
		return nil
	})
	return auth
}

func (m *Map) cleanChars() {
	for range time.Tick(time.Second * 10) {
		m.chmu.Lock()
		for n, c := range m.characters {
			if c.updated.Before(time.Now().Add(-10 * time.Second)) {
				delete(m.characters, n)
			}
		}
		m.chmu.Unlock()
	}
}
