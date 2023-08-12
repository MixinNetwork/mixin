package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
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
	opts = opts.WithMetricsEnabled(false)
	opts = opts.WithLoggingLevel(badger.WARNING)

	// these three options control the maximum database size
	// for level up to max levels: sum(base * (multiplier ** level))
	// increase the level to 8 when data grows big to execeed 16TB
	// the drawback is huge memory usage increases when 1 level up
	// the max levels can be decreased once up, so be cautious
	opts = opts.WithBaseLevelSize(16 << 20)
	opts = opts.WithLevelSizeMultiplier(16)
	opts = opts.WithMaxLevels(7)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	if custom != nil && custom.Storage.ValueLogGC {
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
