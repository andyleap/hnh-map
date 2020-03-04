package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.etcd.io/bbolt"
)

func (m *Map) getChars(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
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
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	markers := []Marker{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("markers"))
		if b == nil {
			return nil
		}
		added := map[string]struct{}{}
		b.ForEach(func(k, v []byte) error {
			ms := []Marker{}
			json.Unmarshal(v, &ms)
			for _, m := range ms {
				pos := fmt.Sprintf("%d_%d", m.Position.X, m.Position.Y)
				if _, ok := added[pos]; !ok {
					markers = append(markers, m)
					added[pos] = struct{}{}
				}
			}
			return nil
		})
		return nil
	})
	json.NewEncoder(rw).Encode(markers)
}
