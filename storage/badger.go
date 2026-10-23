package storage

import (
	"errors"
	"sync"
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
	custodians  sync.Map
	mutex       *sync.RWMutex
	done        chan struct{}
	gcLoops     sync.WaitGroup
	closeOnce   sync.Once
	closeErr    error
}

func NewBadgerStore(custom *config.Custom, dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", true, custom)
	if err != nil {
		return nil, err
	}
	cacheDB, err := openDB(dir+"/cache", false, custom)
	if err != nil {
		return nil, errors.Join(err, snapshotsDB.Close())
	}
	store := &BadgerStore{
		custom:      custom,
		snapshotsDB: snapshotsDB,
		cacheDB:     cacheDB,
		mutex:       new(sync.RWMutex),
		done:        make(chan struct{}),
	}
	if custom != nil && custom.Storage.ValueLogGC {
		for _, db := range []*badger.DB{snapshotsDB, cacheDB} {
			store.gcLoops.Go(func() { store.runValueLogGC(db) })
		}
	}
	return store, nil
}

func (store *BadgerStore) Close() error {
	store.closeOnce.Do(func() {
		if store.done != nil {
			close(store.done)
		}
		store.gcLoops.Wait()
		store.closeErr = errors.Join(store.snapshotsDB.Close(), store.cacheDB.Close())
	})
	return store.closeErr
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
	// increase the level to 8 when data grows big to exceed 16TB
	// the memory usage will increase for hours to compact when level up
	// the max levels can not be decreased once up, so be cautious
	opts = opts.WithBaseLevelSize(16 << 20)
	opts = opts.WithLevelSizeMultiplier(16)
	opts = opts.WithMaxLevels(7)
	if custom != nil && custom.Storage.MaxCompactionLevels > 0 {
		opts = opts.WithMaxLevels(custom.Storage.MaxCompactionLevels)
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (store *BadgerStore) runValueLogGC(db *badger.DB) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		lsm, vlog := db.Size()
		logger.Printf("Badger LSM %d VLOG %d\n", lsm, vlog)
		if lsm > 1024*1024*8 || vlog > 1024*1024*32 {
			err := db.RunValueLogGC(0.5)
			logger.Printf("Badger RunValueLogGC %v\n", err)
		}
		select {
		case <-store.done:
			return
		case <-ticker.C:
		}
	}
}
