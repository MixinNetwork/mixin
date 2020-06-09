package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
)

type BadgerStore struct {
	custom      *config.Custom
	snapshotsDB *badger.DB
	cacheDB     *badger.DB
	queue       *Queue
	closing     bool
}

func NewBadgerStore(custom *config.Custom, dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", true, custom.Storage.ValueLogGC)
	if err != nil {
		return nil, err
	}
	cacheDB, err := openDB(dir+"/cache", true, custom.Storage.ValueLogGC)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{
		custom:      custom,
		snapshotsDB: snapshotsDB,
		cacheDB:     cacheDB,
		queue:       NewQueue(custom),
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
	return store.cacheDB.Close()
}

func openDB(dir string, sync, valueLogGC bool) (*badger.DB, error) {
	opts := badger.DefaultOptions(dir)
	opts = opts.WithSyncWrites(sync)
	opts = opts.WithCompression(options.None)
	opts = opts.WithMaxCacheSize(0)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	if valueLogGC {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				lsm, vlog := db.Size()
				if lsm > 1024*1024*8 || vlog > 1024*1024*32 {
					db.RunValueLogGC(0.5)
				}
			}
		}()
	}

	return db, nil
}
