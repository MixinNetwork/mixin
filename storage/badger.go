package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger"
)

type BadgerStore struct {
	snapshotsDB *badger.DB
	cacheDB     *badger.DB
	stateDB     *badger.DB
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", false)
	if err != nil {
		return nil, err
	}
	cacheDB, err := openDB(dir+"/cache", false)
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
	}, nil
}

func (store *BadgerStore) Close() error {
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
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	opts.SyncWrites = sync
	opts.NumVersionsToKeep = 1
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	db.RunValueLogGC(0.1)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			err := db.RunValueLogGC(0.5)
			logger.Println("badger value log GC", dir, err)
		}
	}()
	return db, nil
}
