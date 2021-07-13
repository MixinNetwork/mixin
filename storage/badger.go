package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
)

type BadgerStore struct {
	custom      *config.Custom
	snapshotsDB *badger.DB
	cacheDB     *badger.DB
	closing     bool
}

func NewBadgerStore(custom *config.Custom, dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", true, custom)
	if err != nil {
		return nil, err
	}
	cacheDB, err := openDB(dir+"/cache", false, custom)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{
		custom:      custom,
		snapshotsDB: snapshotsDB,
		cacheDB:     cacheDB,
		closing:     false,
	}, nil
}

func (store *BadgerStore) Close() error {
	store.closing = true
	err := store.snapshotsDB.Close()
	if err != nil {
		return err
	}
	return store.cacheDB.Close()
}

func openDB(dir string, sync bool, custom *config.Custom) (*badger.DB, error) {
	opts := badger.DefaultOptions(dir)
	opts = opts.WithSyncWrites(sync)
	opts = opts.WithCompression(options.None)
	opts = opts.WithBlockCacheSize(0)
	opts = opts.WithIndexCacheSize(0)
	opts = opts.WithTruncate(!sync || custom.Storage.Truncate)
	opts = opts.WithMaxTableSize(64 << 20)
	opts = opts.WithValueLogFileSize(1024 << 20)
	if custom.Storage.LowMemoryMode {
		opts = opts.WithTableLoadingMode(options.FileIO)
		opts = opts.WithValueLogLoadingMode(options.FileIO)
	}
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	if custom.Storage.ValueLogGC {
		go func() {
			for {
				lsm, vlog := db.Size()
				logger.Printf("Badger LSM %d VLOG %d\n", lsm, vlog)
				if lsm > 1024*1024*8 || vlog > 1024*1024*32 {
					err := db.RunValueLogGC(0.5)
					logger.Printf("Badger RunValueLogGC %v\n", err)
				}
				time.Sleep(5 * time.Minute)
			}
		}()
	}

	return db, nil
}
