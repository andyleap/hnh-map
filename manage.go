package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func (m *Map) index(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil {
		http.Redirect(rw, req, "/login", 302)
		return
	}

	tokens := []string{}
	prefix := "http://example.com"
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		uRaw := b.Get([]byte(s.Username))
		if uRaw == nil {
			return nil
		}
		u := User{}
		json.Unmarshal(uRaw, &u)
		tokens = u.Tokens

		config := tx.Bucket([]byte("config"))
		if config != nil {
			prefix = string(config.Get([]byte("prefix")))
		}
		return nil
	})

	m.ExecuteTemplate(rw, "index.tmpl", struct {
		Page         Page
		Session      *Session
		UploadTokens []string
		Prefix       string
	}{
		Page:         m.getPage(req),
		Session:      s,
		UploadTokens: tokens,
		Prefix:       prefix,
	})
}

func (m *Map) login(rw http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		u := m.getUser(req.FormValue("user"), req.FormValue("pass"))
		if u != nil {
			session := make([]byte, 32)
			rand.Read(session)
			http.SetCookie(rw, &http.Cookie{
				Name:    "session",
				Expires: time.Now().Add(time.Hour * 24 * 7),
				Value:   hex.EncodeToString(session),
			})
			s := &Session{
				ID:        hex.EncodeToString(session),
				Username:  req.FormValue("user"),
				TempAdmin: u.Auths.Has("tempadmin"),
			}
			m.saveSession(s)
			http.Redirect(rw, req, "/", 302)
			return
		}
	}
	m.ExecuteTemplate(rw, "login.tmpl", struct {
		Page Page
	}{
		Page: m.getPage(req),
	})
}

func (m *Map) logout(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s != nil {
		m.deleteSession(s)
	}
	http.Redirect(rw, req, "/login", 302)
	return
}

func (m *Map) generateToken(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_UPLOAD) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	tokenRaw := make([]byte, 16)
	_, err := rand.Read(tokenRaw)
	if err != nil {
		rw.WriteHeader(500)
		return
	}
	token := hex.EncodeToString(tokenRaw)
	m.db.Update(func(tx *bbolt.Tx) error {
		ub, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		uRaw := ub.Get([]byte(s.Username))
		if uRaw == nil {
			return nil
		}
		u := User{}
		err = json.Unmarshal(uRaw, &u)
		if err != nil {
			return err
		}
		u.Tokens = append(u.Tokens, token)
		buf, err := json.Marshal(u)
		if err != nil {
			return err
		}
		err = ub.Put([]byte(s.Username), buf)
		if err != nil {
			return err
		}
		b, err := tx.CreateBucketIfNotExists([]byte("tokens"))
		if err != nil {
			return err
		}
		return b.Put([]byte(token), []byte(s.Username))
	})
	http.Redirect(rw, req, "/", 302)
}

func (m *Map) changePassword(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil {
		http.Redirect(rw, req, "/", 302)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()
		password := req.FormValue("pass")
		m.db.Update(func(tx *bbolt.Tx) error {
			users, err := tx.CreateBucketIfNotExists([]byte("users"))
			if err != nil {
				return err
			}
			u := User{}
			raw := users.Get([]byte(s.Username))
			if raw != nil {
				json.Unmarshal(raw, &u)
			}
			if password != "" {
				u.Pass, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			}
			raw, _ = json.Marshal(u)
			users.Put([]byte(s.Username), raw)
			return nil
		})
		http.Redirect(rw, req, "/", 302)
	}

	m.ExecuteTemplate(rw, "password.tmpl", struct {
		Page    Page
		Session *Session
	}{
		Page:    m.getPage(req),
		Session: s,
	})
}
