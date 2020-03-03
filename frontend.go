package main

import (
	"encoding/json"
	"net/http"

	"go.etcd.io/bbolt"
)

func (m *Map) getChars(rw http.ResponseWriter, req *http.Request) {
	user, pass, _ := req.BasicAuth()
	if !m.getAuth(user, pass).Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	chars := []Character{}
	m.chmu.RLock()
	defer m.chmu.RUnlock()
	for _, v := range m.characters {
		chars = append(chars, v)
	}
	json.NewEncoder(rw).Encode(chars)
}

func (m *Map) getMarkers(rw http.ResponseWriter, req *http.Request) {
	user, pass, _ := req.BasicAuth()
	if !m.getAuth(user, pass).Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	markers := []Marker{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("markers"))
		if b == nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			c := Marker{}
			json.Unmarshal(v, &c)
			markers = append(markers, c)
			return nil
		})
		return nil
	})
	json.NewEncoder(rw).Encode(markers)
}
