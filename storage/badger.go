package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/logger"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/dgraph-io/badger"
)

type BadgerStore struct {
	snapshotsDB *badger.DB
	cacheDB     *badger.DB
	stateDB     *badger.DB
	queue       *Queue
	closing     bool
}

func NewBadgerStore(dir string, cache *fastcache.Cache) (*BadgerStore, error) {
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
		queue:       NewQueue(cacheDB, cache),
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
			lsm, vlog := db.Size()
			if lsm > 1024*1024*8 || vlog > 1024*1024*32 {
				err := db.RunValueLogGC(0.5)
				logger.Println("badger value log GC", dir, err)
			} else {
				logger.Println("badger size", dir, lsm, vlog)
			}
		}
	}()
	return db, nil
}
