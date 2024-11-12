package p2p

import (
	"encoding/binary"
	"slices"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/ristretto/v2"
)

type confirmMap struct {
	cache *ristretto.Cache[[]byte, any]
}

func (m *confirmMap) contains(key []byte, duration time.Duration) bool {
	if key == nil {
		return false
	}
	val, found := m.cache.Get(key)
	if found {
		ts := time.Unix(0, int64(binary.BigEndian.Uint64(val.([]byte))))
		return ts.Add(duration).After(time.Now())
	}
	return false
}

func (m *confirmMap) store(key []byte, ts time.Time) {
	if key == nil {
		panic(ts)
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts.UnixNano()))
	m.cache.Set(key, buf, 8)
}

type remoteRelayer struct {
	Id       crypto.Hash
	ActiveAt time.Time
}

type relayersMap struct {
	sync.RWMutex
	m map[crypto.Hash][]*remoteRelayer
}

func (m *relayersMap) Get(key crypto.Hash) []crypto.Hash {
	m.RLock()
	defer m.RUnlock()

	var relayers []crypto.Hash
	for _, r := range m.m[key] {
		if r.ActiveAt.Add(time.Minute).Before(time.Now()) {
			continue
		}
		relayers = append(relayers, r.Id)
	}
	return relayers
}

func (m *relayersMap) Add(key crypto.Hash, v crypto.Hash) {
	m.Lock()
	defer m.Unlock()

	var relayers []*remoteRelayer
	for _, r := range m.m[key] {
		if r.ActiveAt.Add(time.Minute).After(time.Now()) {
			relayers = append(relayers, r)
		}
	}
	for _, r := range relayers {
		if r.Id == v {
			r.ActiveAt = time.Now()
			return
		}
	}
	i := slices.IndexFunc(relayers, func(r *remoteRelayer) bool {
		return r.Id == v
	})
	if i < 0 {
		relayers = append(relayers, &remoteRelayer{ActiveAt: time.Now(), Id: v})
	} else {
		relayers[i].ActiveAt = time.Now()
	}
	m.m[key] = relayers
}

type neighborMap struct {
	sync.RWMutex
	m map[crypto.Hash]*Peer
}

func (m *neighborMap) Get(key crypto.Hash) *Peer {
	m.RLock()
	defer m.RUnlock()

	return m.m[key]
}

func (m *neighborMap) Delete(key crypto.Hash) {
	m.Lock()
	defer m.Unlock()

	delete(m.m, key)
}

func (m *neighborMap) Set(key crypto.Hash, v *Peer) {
	m.Lock()
	defer m.Unlock()

	m.m[key] = v
}

func (m *neighborMap) Put(key crypto.Hash, v *Peer) bool {
	m.Lock()
	defer m.Unlock()

	if m.m[key] != nil {
		return false
	}
	m.m[key] = v
	return true
}

func (m *neighborMap) Slice() []*Peer {
	m.Lock()
	defer m.Unlock()

	var peers []*Peer
	for _, p := range m.m {
		peers = append(peers, p)
	}
	return peers
}
