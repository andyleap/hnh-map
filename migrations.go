package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

var migrations = []func(tx *bbolt.Tx) error{
	func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("markers")) != nil {
			return tx.DeleteBucket([]byte("markers"))
		}
		return nil
	},
	func(tx *bbolt.Tx) error {
		grids, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
		if err != nil {
			return err
		}
		zoom, err := tiles.CreateBucketIfNotExists([]byte(strconv.Itoa(00)))
		if err != nil {
			return err
		}

		return grids.ForEach(func(k, v []byte) error {
			g := GridData{}
			err := json.Unmarshal(v, &g)
			if err != nil {
				return err
			}
			td := &TileData{
				Coord: g.Coord,
				Zoom:  0,
				File:  fmt.Sprintf("0/%s", g.Coord.Name()),
				Cache: time.Now().UnixNano(),
			}
			raw, err := json.Marshal(td)
			if err != nil {
				return err
			}
			return zoom.Put([]byte(g.Coord.Name()), raw)
		})
	},
	func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("title"), []byte("HnH Automapper Server"))
	},
	func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("markers")) != nil {
			return tx.DeleteBucket([]byte("markers"))
		}
		return nil
	},
	func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("tiles")) != nil {
			allTiles := map[string]map[string]TileData{}
			tiles := tx.Bucket([]byte("tiles"))
			err := tiles.ForEach(func(k, v []byte) error {
				zoom := tiles.Bucket(k)
				zoomTiles := map[string]TileData{}

				allTiles[string(k)] = zoomTiles
				return zoom.ForEach(func(tk, tv []byte) error {
					td := TileData{}
					json.Unmarshal(tv, &td)
					zoomTiles[string(tk)] = td
					return nil
				})
			})
			if err != nil {
				return err
			}
			err = tx.DeleteBucket([]byte("tiles"))
			if err != nil {
				return err
			}
			tiles, err = tx.CreateBucket([]byte("tiles"))
			if err != nil {
				return err
			}
			maptiles, err := tiles.CreateBucket([]byte("0"))
			if err != nil {
				return err
			}
			for k, v := range allTiles {
				zoom, err := maptiles.CreateBucket([]byte(k))
				if err != nil {
					return err
				}
				for tk, tv := range v {
					raw, _ := json.Marshal(tv)
					err = zoom.Put([]byte(strings.TrimSuffix(tk, ".png")), raw)
					if err != nil {
						return err
					}
				}
			}
			err = tiles.SetSequence(1)
			if err != nil {
				return err
			}
		}
		return nil
	},
	func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("markers")) != nil {
			return tx.DeleteBucket([]byte("markers"))
		}
		return nil
	},
	func(tx *bbolt.Tx) error {
		highest := uint64(0)
		maps, err := tx.CreateBucketIfNotExists([]byte("maps"))
		if err != nil {
			return err
		}
		grids, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		mapsFound := map[int]struct{}{}
		err = grids.ForEach(func(k, v []byte) error {
			gd := GridData{}
			err := json.Unmarshal(v, &gd)
			if err != nil {
				return err
			}
			if _, ok := mapsFound[gd.Map]; !ok {
				if uint64(gd.Map) > highest {
					highest = uint64(gd.Map)
				}
				mi := MapInfo{
					ID:     gd.Map,
					Name:   strconv.Itoa(gd.Map),
					Hidden: false,
				}
				raw, _ := json.Marshal(mi)
				return maps.Put([]byte(strconv.Itoa(gd.Map)), raw)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return maps.SetSequence(highest + 1)
	},
	func(tx *bbolt.Tx) error {
		users := tx.Bucket([]byte("users"))
		if users == nil {
			return nil
		}
		return users.ForEach(func(k, v []byte) error {
			u := User{}
			json.Unmarshal(v, &u)
			if u.Auths.Has(AUTH_MAP) && !u.Auths.Has(AUTH_MARKERS) {
				u.Auths = append(u.Auths, AUTH_MARKERS)
				raw, err := json.Marshal(u)
				if err != nil {
					return err
				}
				users.Put(k, raw)
			}
			return nil
		})
	},
}
