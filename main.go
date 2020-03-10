package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/andyleap/hnh-map/webapp"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

type Map struct {
	gridStorage string
	db          *bbolt.DB

	characters map[string]Character
	chmu       sync.RWMutex

	sessions map[string]*Session
	sessmu   sync.RWMutex

	*webapp.WebApp

	gridUpdates topic
}

type Session struct {
	Username string
	Auths    Auths
}

var (
	gridStorage = flag.String("grids", "grids", "directory to store grids in")
	port        = flag.Int("port", func() int {
		if port, ok := os.LookupEnv("HNHMAP_PORT"); ok {
			p, err := strconv.Atoi(port)
			if err != nil {
				log.Fatal(err)
			}
			return p
		}
		return 8080
	}(), "Port to listen on")
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

		sessions: map[string]*Session{},

		WebApp: webapp.Must(webapp.New().LoadTemplates("./templates/")),
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		vraw := b.Get([]byte("version"))
		v, _ := strconv.Atoi(string(vraw))
		if v < len(migrations) {
			for _, f := range migrations[v:] {
				err = f(tx)
				if err != nil {
					return err
				}
			}
		}
		return b.Put([]byte("version"), []byte(strconv.Itoa(len(migrations))))
	})
	if err != nil {
		log.Fatal(err)
	}

	go m.cleanChars()

	// Mapping client endpoints
	http.HandleFunc("/client/", m.client)

	http.HandleFunc("/login", m.login)
	http.HandleFunc("/", m.index)
	http.HandleFunc("/generateToken", m.generateToken)
	http.HandleFunc("/password", m.changePassword)

	// Admin endpoints
	http.HandleFunc("/admin/", m.admin)
	http.HandleFunc("/admin/user", m.adminUser)
	http.HandleFunc("/admin/deleteUser", m.deleteUser)
	http.HandleFunc("/admin/wipe", m.wipe)
	http.HandleFunc("/admin/setPrefix", m.setPrefix)
	http.HandleFunc("/admin/setTitle", m.setTitle)
	http.HandleFunc("/admin/rebuildZooms", m.rebuildZooms)
	http.HandleFunc("/admin/backup", m.backup)
	http.HandleFunc("/admin/merge", m.merge)

	// Map frontend endpoints
	http.HandleFunc("/map/api/v1/characters", m.getChars)
	http.HandleFunc("/map/api/v1/markers", m.getMarkers)
	http.HandleFunc("/map/api/config", m.config)
	http.HandleFunc("/map/api/admin/wipeTile", m.wipeTile)
	http.HandleFunc("/map/api/admin/hideMarker", m.hideMarker)
	http.HandleFunc("/map/updates", m.watchGridUpdates)
	http.HandleFunc("/map/grids/", m.gridTile)
	//http.Handle("/map/grids/", http.StripPrefix("/map/grids", http.FileServer(http.Dir(m.gridStorage))))

	http.Handle("/map/", http.StripPrefix("/map", http.FileServer(http.Dir("frontend"))))

	log.Printf("Listening on port %d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
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
	GridID   int      `json:"string"`
	Position Position `json:"position"`
	Image    string   `json:"image"`
	Hidden   bool     `json:"hidden"`
}

type FrontendMarker struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	Position Position `json:"position"`
	Image    string   `json:"image"`
	Hidden   bool     `json:"hidden"`
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

func (c Coord) Parent() Coord {
	if c.X < 0 {
		c.X--
	}
	if c.Y < 0 {
		c.Y--
	}
	return Coord{
		X: c.X / 2,
		Y: c.Y / 2,
	}
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
	Pass   []byte
	Auths  Auths
	Tokens []string
}

func (m *Map) getSession(req *http.Request) *Session {
	m.sessmu.RLock()
	defer m.sessmu.RUnlock()
	c, err := req.Cookie("session")
	if err != nil {
		return nil
	}
	return m.sessions[c.Value]
}

type Page struct {
	Title string `json:"title"`
}

func (m *Map) getPage(req *http.Request) Page {
	p := Page{}
	m.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte("config"))
		if c == nil {
			return nil
		}
		p.Title = string(c.Get([]byte("title")))
		return nil
	})
	return p
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
