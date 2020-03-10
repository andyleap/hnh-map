package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
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
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		config := tx.Bucket([]byte("config"))
		if config != nil {
			prefix = string(config.Get([]byte("prefix")))
		}
		return b.ForEach(func(k, v []byte) error {
			users = append(users, string(k))
			return nil
		})
	})

	m.ExecuteTemplate(rw, "admin/index.tmpl", struct {
		Page    Page
		Session *Session
		Users   []string
		Prefix  string
	}{
		Page:    m.getPage(req),
		Session: s,
		Users:   users,
		Prefix:  prefix,
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
			if len(auths) > 0 {
				u.Auths = auths
			}
			raw, _ = json.Marshal(u)
			users.Put([]byte(username), raw)
			return nil
		})
		if username == s.Username {
			s.Auths = auths
		}
		if tempAdmin {
			m.sessmu.Lock()
			defer m.sessmu.Unlock()
			s, _ := req.Cookie("session")
			delete(m.sessions, s.Value)
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
	m.db.Update(func(tx *bbolt.Tx) error {
		tx.DeleteBucket([]byte("grids"))
		tx.DeleteBucket([]byte("markers"))
		tx.DeleteBucket([]byte("tiles"))
		return nil
	})
	for z := 0; z <= 5; z++ {
		os.RemoveAll(fmt.Sprintf("%s/%d", m.gridStorage, z))
	}
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

func (m *Map) rebuildZooms(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	needProcess := map[Coord]struct{}{}

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
			needProcess[grid.Coord.Parent()] = struct{}{}
			return nil
		})
		return nil
	})

	if noGrids {
		return
	}

	for z := 1; z <= 5; z++ {
		os.RemoveAll(fmt.Sprintf("%s/%d", m.gridStorage, z))
		process := needProcess
		needProcess = map[Coord]struct{}{}
		for c := range process {
			m.updateZoomLevel(c, z)
			needProcess[c.Parent()] = struct{}{}
		}
	}
	m.gridUpdates.close()
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
		m.sessmu.Lock()
		defer m.sessmu.Unlock()
		s, _ := req.Cookie("session")
		delete(m.sessions, s.Value)
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
		id := []byte{}
		err := grids.ForEach(func(k, v []byte) error {
			g := GridData{}
			err := json.Unmarshal(v, &g)
			if err != nil {
				return err
			}
			if g.Coord == c {
				id = k
				return errFound
			}
			return nil
		})
		if err != errFound {
			return err
		}
		grids.Delete(id)

		return nil
	})

	m.SaveTile(c, 0, "", -1)
	for z := 1; z <= 5; z++ {
		c = c.Parent()
		m.updateZoomLevel(c, z)
	}
	http.Redirect(rw, req, "/admin/", 302)
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
	dbf, err := ioutil.TempFile("", "grids*.db")
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	for _, fhdr := range zr.File {
		if fhdr.Name == "grids.db" {
			r, err := fhdr.Open()
			if err != nil {
				dbf.Close()
				log.Println(err)
				http.Error(rw, "internal error", http.StatusInternalServerError)
				return
			}
			_, err = io.Copy(dbf, r)
			if err != nil {
				dbf.Close()
				log.Println(err)
				http.Error(rw, "internal error", http.StatusInternalServerError)
				return
			}
			r.Close()
			break
		}
	}

	dbname := dbf.Name()
	dbf.Close()
	db, err := bbolt.Open(dbname, 0600, nil)
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() {
		db.Close()
		os.Remove(dbf.Name())
	}()

	ops := []struct {
		x, y int
		f    string
	}{}

	err = db.View(func(mtx *bbolt.Tx) error {
		return m.db.Update(func(ttx *bbolt.Tx) error {
			{
				mv := 0
				{
					b := mtx.Bucket([]byte("config"))
					if b == nil {
						return fmt.Errorf("no config bucket")
					}
					vraw := b.Get([]byte("version"))
					mv, _ = strconv.Atoi(string(vraw))
				}
				tv := 0
				{
					b, err := ttx.CreateBucketIfNotExists([]byte("config"))
					if err != nil {
						return err
					}
					vraw := b.Get([]byte("version"))
					tv, _ = strconv.Atoi(string(vraw))
				}
				if mv != tv {
					return fmt.Errorf("Version %d does not match %d", mv, tv)
				}
			}
			locked := false
			offset := Coord{}
			mgrids := mtx.Bucket([]byte("grids"))
			tgrids := ttx.Bucket([]byte("grids"))
			if mgrids == nil {
				return fmt.Errorf("Merge source grids missing, cancelling merge")
			}
			if tgrids != nil {
				err = mgrids.ForEach(func(k, v []byte) error {
					tgrid := tgrids.Get(k)
					if tgrid != nil {
						tg := GridData{}
						mg := GridData{}
						json.Unmarshal(tgrid, &tg)
						json.Unmarshal(v, &mg)
						locked = true
						offset.X = tg.Coord.X - mg.Coord.X
						offset.Y = tg.Coord.Y - mg.Coord.Y
						return errFound
					}
					return nil
				})
				if err != errFound && err != nil {
					return err
				}
			} else {
				locked = true
			}
			if !locked {
				return fmt.Errorf("Map grids do not intersect, cancelling merge")
			}

			ttiles, err := ttx.CreateBucketIfNotExists([]byte("tiles"))
			if err != nil {
				return err
			}
			tzoom, err := ttiles.CreateBucketIfNotExists([]byte("0"))
			if err != nil {
				return err
			}
			mtiles := mtx.Bucket([]byte("tiles"))
			if ttiles == nil {
				return nil
			}
			mzoom := mtiles.Bucket([]byte("0"))
			if tzoom == nil {
				return nil
			}

			err = mgrids.ForEach(func(k, v []byte) error {
				mg := GridData{}
				json.Unmarshal(v, &mg)

				td := TileData{}
				tileraw := mzoom.Get([]byte(mg.Coord.Name()))
				if tileraw == nil {
					return nil
				}
				json.Unmarshal(tileraw, &td)
				for _, tf := range zr.File {
					if tf.Name == td.File {
						tfr, err := tf.Open()
						if err != nil {
							return err
						}
						defer tfr.Close()
						tfw, err := os.Create(m.gridStorage + "/0/" + string(k) + ".png")
						ops = append(ops, struct {
							x int
							y int
							f string
						}{
							x: td.Coord.X + offset.X,
							y: td.Coord.Y + offset.Y,
							f: "0/" + string(k) + ".png",
						})
						if err != nil {
							return err
						}
						_, err = io.Copy(tfw, tfr)
						if err != nil {
							return err
						}
						break
					}
				}

				mg.Coord.X += offset.X
				mg.Coord.Y += offset.Y
				raw, _ := json.Marshal(mg)
				err := tgrids.Put(k, raw)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				return err
			}

			return nil
		})
	})

	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	for _, op := range ops {
		m.SaveTile(Coord{X: op.x, Y: op.y}, 0, op.f, time.Now().UnixNano())
	}
	m.rebuildZooms(rw, req)
}
