package storage

import (
	"github.com/dgraph-io/badger"
)

type BadgerStore struct {
	snapshotsDB *badger.DB
	queueDB     *badger.DB
	stateDB     *badger.DB
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", false)
	if err != nil {
		return nil, err
	}
	queueDB, err := openDB(dir+"/queue", false)
	if err != nil {
		return nil, err
	}
	stateDB, err := openDB(dir+"/state", true)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{
		snapshotsDB: snapshotsDB,
		queueDB:     queueDB,
		stateDB:     stateDB,
	}, nil
}

func openDB(dir string, sync bool) (*badger.DB, error) {
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	opts.SyncWrites = sync
	opts.NumVersionsToKeep = 1
	return badger.Open(opts)
}
