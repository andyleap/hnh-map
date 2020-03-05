package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

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
		Session *Session
		Users   []string
		Prefix  string
	}{
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
		Session  *Session
		User     User
		Username string
	}{
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
