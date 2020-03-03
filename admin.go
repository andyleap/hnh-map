package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func (m *Map) setZero(rw http.ResponseWriter, req *http.Request) {
	user, pass, _ := req.BasicAuth()
	if !m.getAuth(user, pass).Has(AUTH_ADMIN) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
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

func (m *Map) setUser(rw http.ResponseWriter, req *http.Request) {
	authUser, authPass, _ := req.BasicAuth()
	if !m.getAuth(authUser, authPass).Has(AUTH_ADMIN) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	username := req.FormValue("user")
	password := req.FormValue("pass")
	auth := req.FormValue("auths")
	auths := strings.Split(auth, ",")
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
}
