package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func (m *Map) wipe(rw http.ResponseWriter, req *http.Request) {
	user, pass, _ := req.BasicAuth()
	if !m.getAuth(user, pass).Has(AUTH_ADMIN) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		tx.DeleteBucket([]byte("grids"))
		tx.DeleteBucket([]byte("markers"))
		return nil
	})
	for z := 0; z <= 5; z++ {
		os.RemoveAll(fmt.Sprintf("%s/%d", m.gridStorage, z))
	}
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
