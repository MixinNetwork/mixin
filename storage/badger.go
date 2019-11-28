package storage

import (
	"github.com/dgraph-io/badger/v2"
)

type BadgerStore struct {
	snapshotsDB *badger.DB
	cacheDB     *badger.DB
	stateDB     *badger.DB
	queue       *Queue
	closing     bool
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", true)
	if err != nil {
		return nil, err
	}
	cacheDB, err := openDB(dir+"/cache", true)
	if err != nil {
		return nil, err
	}
	stateDB, err := openDB(dir+"/state", true)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{
		snapshotsDB: snapshotsDB,
		cacheDB:     cacheDB,
		stateDB:     stateDB,
		queue:       NewQueue(),
		closing:     false,
	}, nil
}

func (store *BadgerStore) Close() error {
	store.closing = true
	store.queue.Dispose()
	err := store.snapshotsDB.Close()
	if err != nil {
		return err
	}
	err = store.stateDB.Close()
	if err != nil {
		return err
	}
	return store.cacheDB.Close()
}

func openDB(dir string, sync bool) (*badger.DB, error) {
	opts := badger.DefaultOptions(dir)
	opts.SyncWrites = sync
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return db, nil
}
