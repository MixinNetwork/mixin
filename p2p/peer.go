package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/util"
	"github.com/dgraph-io/ristretto"
)

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	sentMetric     *MetricPool
	receivedMetric *MetricPool

	ctx             context.Context
	handle          SyncHandle
	relayers        *neighborMap
	consumers       *neighborMap
	snapshotsCaches *confirmMap
	highRing        *util.RingBuffer
	normalRing      *util.RingBuffer
	syncRing        *util.RingBuffer
	closing         bool
	ops             chan struct{}
	stn             chan struct{}

	relayer         *QuicRelayer
	consumerAuth    *AuthToken
	isRemoteRelayer bool
	remoteRelayers  *neighborMap
}

type SyncPoint struct {
	NodeId crypto.Hash `json:"node"`
	Number uint64      `json:"round"`
	Hash   crypto.Hash `json:"hash"`
	Pool   any         `json:"pool"`
}

type ChanMsg struct {
	key  []byte
	data []byte
}

func (me *Peer) ConnectRelayer(idForNetwork crypto.Hash, addr string) {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		panic(fmt.Errorf("invalid address %s %s", addr, err))
	} else if a.Port < 80 || a.IP == nil {
		panic(fmt.Errorf("invalid address %s %d %s", addr, a.Port, a.IP))
	}
	if me.isRemoteRelayer {
		me.remoteRelayers = &neighborMap{m: make(map[crypto.Hash]*Peer)}
	}

	for !me.closing {
		time.Sleep(time.Duration(config.SnapshotRoundGap))
		old := me.relayers.Get(idForNetwork)
		if old != nil {
			panic(fmt.Errorf("ConnectRelayer(%s) => %s", idForNetwork, old.Address))
		}
		relayer := NewPeer(nil, idForNetwork, addr, true)
		err := me.connectRelayer(relayer)
		logger.Printf("me.connectRelayer(%s, %v) => %v", me.Address, relayer, err)
	}
}

func (me *Peer) connectRelayer(relayer *Peer) error {
	logger.Printf("me.connectRelayer(%s, %s) => %v", me.Address, me.IdForNetwork, relayer)
	client, err := NewQuicConsumer(me.ctx, relayer.Address)
	logger.Printf("NewQuicConsumer(%s) => %v %v", relayer.Address, client, err)
	if err != nil {
		return err
	}
	defer client.Close("connectRelayer")
	defer relayer.disconnect()

	auth := me.handle.BuildAuthenticationMessage(relayer.IdForNetwork)
	err = client.Send(buildAuthenticationMessage(auth))
	logger.Printf("client.SendAuthenticationMessage(%x) => %v", auth, err)
	if err != nil {
		return err
	}
	me.sentMetric.handle(PeerMessageTypeAuthentication)
	if !me.relayers.Put(relayer.IdForNetwork, relayer) {
		panic(fmt.Errorf("ConnectRelayer(%s) => %s", relayer.IdForNetwork, relayer.Address))
	}
	defer me.relayers.Delete(relayer.IdForNetwork)

	go me.syncToNeighborLoop(relayer)
	go me.loopReceiveMessage(relayer, client)
	_, err = me.loopSendingStream(relayer, client)
	logger.Printf("me.loopSendingStream(%s, %s) => %v", me.Address, client.RemoteAddr().String(), err)
	return err
}

func (me *Peer) Neighbors() []*Peer {
	relayers := me.relayers.Slice()
	consumers := me.consumers.Slice()
	return append(relayers, consumers...)
}

func (p *Peer) disconnect() {
	if p.closing {
		return
	}
	p.closing = true
	p.highRing.Dispose()
	p.normalRing.Dispose()
	p.syncRing.Dispose()
	<-p.ops
	<-p.stn
}

func (me *Peer) Metric() map[string]*MetricPool {
	metrics := make(map[string]*MetricPool)
	if me.sentMetric.enabled {
		metrics["sent"] = me.sentMetric
	}
	if me.receivedMetric.enabled {
		metrics["received"] = me.receivedMetric
	}
	return metrics
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string, isRelayer bool) *Peer {
	peer := &Peer{
		IdForNetwork:    idForNetwork,
		Address:         addr,
		relayers:        &neighborMap{m: make(map[crypto.Hash]*Peer)},
		consumers:       &neighborMap{m: make(map[crypto.Hash]*Peer)},
		highRing:        util.NewRingBuffer(1024),
		normalRing:      util.NewRingBuffer(1024),
		syncRing:        util.NewRingBuffer(1024),
		handle:          handle,
		sentMetric:      &MetricPool{enabled: false},
		receivedMetric:  &MetricPool{enabled: false},
		ops:             make(chan struct{}),
		stn:             make(chan struct{}),
		isRemoteRelayer: isRelayer,
	}
	peer.ctx = context.Background() // FIXME use real context
	if handle != nil {
		peer.snapshotsCaches = &confirmMap{cache: handle.GetCacheStore()}
	}
	return peer
}

func (me *Peer) Teardown() {
	me.closing = true
	if me.relayer != nil {
		me.relayer.Close()
	}
	me.highRing.Dispose()
	me.normalRing.Dispose()
	me.syncRing.Dispose()
	peers := me.Neighbors()
	var wg sync.WaitGroup
	for _, p := range peers {
		wg.Add(1)
		go func(p *Peer) {
			p.disconnect()
			wg.Done()
		}(p)
	}
	wg.Wait()
	logger.Printf("Teardown(%s, %s)\n", me.IdForNetwork, me.Address)
}

func (me *Peer) ListenConsumers() error {
	logger.Printf("me.ListenConsumers(%s, %s)", me.Address, me.IdForNetwork)
	relayer, err := NewQuicRelayer(me.Address)
	if err != nil {
		return err
	}
	me.relayer = relayer
	me.remoteRelayers = &neighborMap{m: make(map[crypto.Hash]*Peer)}

	go func() {
		ticker := time.NewTicker(time.Duration(config.SnapshotRoundGap))
		defer ticker.Stop()

		for !me.closing {
			neighbors := me.Neighbors()
			msg := me.buildConsumersMessage()
			for _, p := range neighbors {
				if !p.isRemoteRelayer {
					continue
				}
				key := crypto.Blake3Hash(append(msg, p.IdForNetwork[:]...))
				me.sendHighToPeer(p.IdForNetwork, PeerMessageTypeConsumers, key[:], msg)
			}

			<-ticker.C
		}
	}()

	for !me.closing {
		c, err := me.relayer.Accept(me.ctx)
		logger.Printf("me.relayer.Accept(%s) => %v %v", me.Address, c, err)
		if err != nil {
			continue
		}
		go func(c Client) {
			defer c.Close("authenticateNeighbor")

			peer, err := me.authenticateNeighbor(c)
			logger.Printf("me.authenticateNeighbor(%s, %s) => %v %v", me.Address, c.RemoteAddr().String(), peer, err)
			if err != nil {
				return
			}
			defer peer.disconnect()
			if !me.consumers.Put(peer.IdForNetwork, peer) {
				return
			}
			defer me.consumers.Delete(peer.IdForNetwork)

			go me.syncToNeighborLoop(peer)
			go me.loopReceiveMessage(peer, c)
			_, err = me.loopSendingStream(peer, c)
			logger.Printf("me.loopSendingStream(%s, %s) => %v", me.Address, c.RemoteAddr().String(), err)
		}(c)
	}

	logger.Printf("ListenConsumers(%s, %s) DONE\n", me.IdForNetwork, me.Address)
	return nil
}

func (me *Peer) loopSendingStream(p *Peer, consumer Client) (*ChanMsg, error) {
	defer close(p.ops)
	defer consumer.Close("loopSendingStream")

	graphTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap / 2))
	defer graphTicker.Stop()

	for !me.closing && !p.closing {
		msgs := []*ChanMsg{}
		select {
		case <-graphTicker.C:
			me.sentMetric.handle(PeerMessageTypeGraph)
			msg := buildGraphMessage(me.handle)
			msgs = append(msgs, &ChanMsg{nil, msg})
		default:
		}

		for len(msgs) < 16 {
			item, err := p.highRing.Poll(false)
			if err != nil {
				return nil, err
			} else if item == nil {
				break
			}
			msg := item.(*ChanMsg)
			if me.snapshotsCaches.contains(msg.key, time.Minute) {
				continue
			}
			msgs = append(msgs, msg)
		}

		for len(msgs) < 16 {
			item, err := p.normalRing.Poll(false)
			if err != nil {
				return nil, err
			} else if item == nil {
				break
			}
			msg := item.(*ChanMsg)
			if me.snapshotsCaches.contains(msg.key, time.Minute) {
				continue
			}
			msgs = append(msgs, msg)
		}

		if len(msgs) == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, m := range msgs {
			if m.key != nil && me.snapshotsCaches.contains(m.key, time.Minute) {
				continue
			}
			err := consumer.Send(m.data)
			if err != nil {
				return m, fmt.Errorf("consumer.Send(%s, %d) => %v", p.Address, len(m.data), err)
			}
			if m.key != nil {
				me.snapshotsCaches.store(m.key, time.Now())
			}
		}
	}

	return nil, fmt.Errorf("PEER DONE")
}

func (me *Peer) loopReceiveMessage(peer *Peer, client Client) {
	logger.Printf("me.loopReceiveMessage(%s, %s)", me.Address, client.RemoteAddr().String())
	receive := make(chan *PeerMessage, 1024)
	defer close(receive)
	defer client.Close("loopReceiveMessage")

	go func() {
		defer client.Close("handlePeerMessage")

		for msg := range receive {
			err := me.handlePeerMessage(peer.IdForNetwork, msg)
			if err == nil {
				continue
			}
			logger.Printf("me.handlePeerMessage(%s) => %v", peer.IdForNetwork, err)
			return
		}
	}()

	for !me.closing {
		tm, err := client.Receive()
		if err != nil {
			logger.Printf("client.Receive %s %v", peer.Address, err)
			return
		}
		msg, err := parseNetworkMessage(tm.Version, tm.Data)
		if err != nil {
			logger.Debugf("parseNetworkMessage %s %v", peer.Address, err)
			return
		}
		me.receivedMetric.handle(msg.Type)

		select {
		case receive <- msg:
		default:
			logger.Printf("peer receive timeout %s", peer.Address)
			return
		}
	}
}

func (me *Peer) authenticateNeighbor(client Client) (*Peer, error) {
	var peer *Peer
	auth := make(chan error)
	go func() {
		tm, err := client.Receive()
		if err != nil {
			auth <- err
			return
		}
		msg, err := parseNetworkMessage(tm.Version, tm.Data)
		if err != nil {
			auth <- err
			return
		}
		if msg.Type != PeerMessageTypeAuthentication {
			auth <- fmt.Errorf("peer authentication invalid message type %d", msg.Type)
			return
		}
		me.receivedMetric.handle(PeerMessageTypeAuthentication)

		token, err := me.handle.AuthenticateAs(me.IdForNetwork, msg.Data, int64(HandshakeTimeout/time.Second))
		if err != nil {
			auth <- err
			return
		}

		addr := client.RemoteAddr().String()
		peer = NewPeer(nil, token.PeerId, addr, token.IsRelayer)
		peer.consumerAuth = token
		auth <- nil
	}()

	select {
	case err := <-auth:
		if err != nil {
			return nil, err
		}
	case <-time.After(3 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
	return peer, nil
}

func (me *Peer) sendHighToPeer(to crypto.Hash, typ byte, key, data []byte) error {
	return me.sendToPeer(to, typ, key, data, MsgPriorityHigh)
}

func (p *Peer) offer(priority int, msg *ChanMsg) (bool, error) {
	switch priority {
	case MsgPriorityNormal:
		return p.normalRing.Offer(msg)
	case MsgPriorityHigh:
		return p.highRing.Offer(msg)
	}
	panic(priority)
}

func (me *Peer) sendToPeer(to crypto.Hash, typ byte, key, data []byte, priority int) error {
	if to == me.IdForNetwork {
		return nil
	}
	if me.snapshotsCaches.contains(key, time.Minute) {
		return nil
	}
	me.sentMetric.handle(typ)

	peer := me.consumers.Get(to)
	if peer == nil {
		peer = me.relayers.Get(to)
	}
	if peer != nil {
		success, _ := peer.offer(priority, &ChanMsg{key, data})
		if !success {
			return fmt.Errorf("peer send %d timeout", priority)
		}
		return nil
	}

	rm := me.buildRelayMessage(to, data)
	if me.remoteRelayers != nil {
		peer = me.remoteRelayers.Get(to)
	}
	if peer != nil {
		rk := crypto.Blake3Hash(append(rm, peer.IdForNetwork[:]...))
		success, _ := peer.offer(priority, &ChanMsg{rk[:], rm})
		if !success {
			return fmt.Errorf("peer.offer(%s, %s) => %d timeout", peer.Address, peer.IdForNetwork, priority)
		}
		return nil
	}

	neighbors := me.Neighbors()
	for _, peer := range neighbors {
		if !peer.isRemoteRelayer {
			continue
		}
		rk := crypto.Blake3Hash(append(rm, peer.IdForNetwork[:]...))
		success, _ := peer.offer(priority, &ChanMsg{rk[:], rm})
		if success {
			break
		}
		return fmt.Errorf("peer.offer(%s, %s) => %d timeout", peer.Address, peer.IdForNetwork, priority)
	}
	return nil
}

func (me *Peer) sendSnapshotMessageToPeer(to crypto.Hash, snap crypto.Hash, typ byte, data []byte) error {
	key := append(to[:], snap[:]...)
	key = append(key, 'S', 'N', 'A', 'P', typ)
	return me.sendToPeer(to, typ, key, data, MsgPriorityNormal)
}

type confirmMap struct {
	cache *ristretto.Cache
}

func (m *confirmMap) contains(key []byte, duration time.Duration) bool {
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

func (m *neighborMap) Clear() {
	m.Lock()
	defer m.Unlock()

	for id := range m.m {
		delete(m.m, id)
	}
}

func (m *neighborMap) RunOnce(key crypto.Hash, v *Peer, f func()) {
	m.Lock()
	defer m.Unlock()

	if m.m[key] != nil {
		return
	}
	m.m[key] = v
	go f()
}
