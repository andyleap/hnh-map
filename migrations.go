package main

import "go.etcd.io/bbolt"

var migrations = []func(tx *bbolt.Tx) error{
	func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("markers")) != nil {
			return tx.DeleteBucket([]byte("markers"))
		}
		return nil
	},
}
