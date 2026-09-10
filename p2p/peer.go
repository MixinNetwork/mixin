package p2p

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/util"
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
	highRing        chan *ChanMsg
	normalRing      chan *ChanMsg
	syncRing        chan []*SyncPoint
	closing         atomic.Bool
	ops             chan struct{}
	stn             chan struct{}
	sendWake        chan struct{}

	relayer        atomic.Pointer[QuicRelayer]
	authentication *AuthToken // Immutable after this peer is added to a neighbor map.
	isRelayer      bool
	remoteRelayers *relayersMap
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

// artificialSendDelay simulates a wide area network round trip in tests,
// enabled with MIXIN_P2P_ARTIFICIAL_DELAY, e.g. "150ms". It applies to
// every message hop on the sending path. Zero in production.
var artificialSendDelay = func() time.Duration {
	d, _ := time.ParseDuration(os.Getenv("MIXIN_P2P_ARTIFICIAL_DELAY"))
	return d
}()

func (me *Peer) IsRelayer() bool {
	return me.isRelayer
}

func (me *Peer) ConnectRelayer(idForNetwork crypto.Hash, addr string) {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		panic(fmt.Errorf("invalid address %s %s", addr, err))
	} else if a.Port < 80 || a.IP == nil {
		panic(fmt.Errorf("invalid address %s %d %s", addr, a.Port, a.IP))
	}
	for !me.closing.Load() {
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

	if err := me.authenticateRelayer(relayer, client); err != nil {
		return err
	}
	defer relayer.disconnect()
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

func (me *Peer) authenticateRelayer(relayer *Peer, client Client) error {
	auth := me.handle.BuildAuthenticationMessage(relayer.IdForNetwork)
	if err := client.Send(buildAuthenticationMessage(auth)); err != nil {
		return err
	}
	me.sentMetric.handle(PeerMessageTypeAuthentication)
	token, err := me.receiveAuthentication(client)
	if err != nil {
		return err
	}
	if token.PeerId != relayer.IdForNetwork {
		return fmt.Errorf("relayer authentication id mismatch %s %s", token.PeerId, relayer.IdForNetwork)
	}
	if !token.IsRelayer {
		return fmt.Errorf("peer %s is not a relayer", token.PeerId)
	}
	relayer.authentication = token
	return nil
}

func (me *Peer) verifyNeighborSignature(peerId crypto.Hash, data []byte, sig *crypto.Signature) bool {
	if sig == nil {
		return false
	}
	hash := crypto.Blake3Hash(data)
	for _, peer := range me.GetNeighbors(peerId) {
		auth := peer.authentication
		if auth != nil && auth.PeerId == peerId && auth.PublicSpendKey.Verify(hash, *sig) {
			return true
		}
	}
	return false
}

func (me *Peer) Neighbors() []*Peer {
	relayers := me.relayers.Slice()
	consumers := me.consumers.Slice()
	return append(relayers, consumers...)
}

func (p *Peer) disconnect() {
	if !p.closing.CompareAndSwap(false, true) {
		return
	}
	<-p.ops
	<-p.stn
}

func (me *Peer) Metric() map[string]*MetricPool {
	metrics := make(map[string]*MetricPool)
	if me.sentMetric.enabled.Load() {
		metrics["sent"] = me.sentMetric
	}
	if me.receivedMetric.enabled.Load() {
		metrics["received"] = me.receivedMetric
	}
	return metrics
}

func (me *Peer) SetMetricEnabled(enabled bool) {
	me.sentMetric.enabled.Store(enabled)
	me.receivedMetric.enabled.Store(enabled)
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string, isRelayer bool) *Peer {
	ringSize := uint64(1024)
	peer := &Peer{
		IdForNetwork:   idForNetwork,
		Address:        addr,
		relayers:       &neighborMap{m: make(map[crypto.Hash]*Peer)},
		consumers:      &neighborMap{m: make(map[crypto.Hash]*Peer)},
		highRing:       make(chan *ChanMsg, ringSize),
		normalRing:     make(chan *ChanMsg, ringSize),
		syncRing:       make(chan []*SyncPoint, ringSize),
		handle:         handle,
		sentMetric:     &MetricPool{},
		receivedMetric: &MetricPool{},
		ops:            make(chan struct{}),
		stn:            make(chan struct{}),
		sendWake:       make(chan struct{}, 1),
		isRelayer:      isRelayer,
	}
	peer.ctx = context.Background() // FIXME use real context
	if isRelayer {
		peer.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	}
	if handle != nil {
		peer.snapshotsCaches = &confirmMap{cache: handle.GetCacheStore()}
	}
	return peer
}

func (me *Peer) Teardown() {
	if !me.closing.CompareAndSwap(false, true) {
		return
	}
	if relayer := me.relayer.Load(); relayer != nil {
		util.CloseOrPanic(relayer)
	}
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
	me.relayer.Store(relayer)

	go func() {
		for !me.closing.Load() {
			neighbors := me.Neighbors()
			msg := me.buildConsumersMessage()
			for _, p := range neighbors {
				if !p.isRelayer {
					continue
				}
				me.offerToPeerWithCacheCheck(p, MsgPriorityNormal, &ChanMsg{nil, msg})
			}

			time.Sleep(time.Duration(config.SnapshotRoundGap))
		}
	}()

	for !me.closing.Load() {
		c, err := relayer.Accept(me.ctx)
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

			old := me.consumers.Get(peer.IdForNetwork)
			if old != nil {
				old.disconnect()
				me.consumers.Delete(old.IdForNetwork)
			}
			if !me.consumers.Put(peer.IdForNetwork, peer) {
				panic(peer.IdForNetwork)
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

	for !me.closing.Load() && !p.closing.Load() {
		hm := me.pollRingWithCache(p.highRing, 16)
		nm := me.pollRingWithCache(p.normalRing, 16)
		msgs := append(hm, nm...)

		if len(msgs) == 0 {
			p.waitSendingStream(300 * time.Millisecond)
			continue
		}

		if artificialSendDelay > 0 {
			// test-only WAN simulation, enabled with MIXIN_P2P_ARTIFICIAL_DELAY
			time.Sleep(artificialSendDelay)
		}
		for _, m := range msgs {
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

func (me *Peer) pollRingWithCache(ring chan *ChanMsg, limit int) []*ChanMsg {
	var msgs []*ChanMsg
	for len(msgs) < limit {
		select {
		case msg := <-ring:
			if me.snapshotsCaches.contains(msg.key, time.Minute) {
				continue
			}
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
	return msgs
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

	for !me.closing.Load() {
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
	token, err := me.receiveAuthentication(client)
	if err != nil {
		return nil, err
	}
	auth := me.handle.BuildAuthenticationMessage(token.PeerId)
	if err := client.Send(buildAuthenticationMessage(auth)); err != nil {
		return nil, err
	}
	me.sentMetric.handle(PeerMessageTypeAuthentication)
	peer := NewPeer(nil, token.PeerId, client.RemoteAddr().String(), token.IsRelayer)
	peer.authentication = token
	return peer, nil
}

func (me *Peer) receiveAuthentication(client Client) (*AuthToken, error) {
	var token *AuthToken
	auth := make(chan error, 1)
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

		token, err = me.handle.AuthenticateAs(me.IdForNetwork, msg.Data, int64(HandshakeTimeout/time.Second))
		auth <- err
	}()

	select {
	case err := <-auth:
		if err != nil {
			return nil, err
		}
	case <-time.After(3 * time.Second):
		client.Close("authenticate timeout")
		return nil, fmt.Errorf("authenticate timeout")
	}
	return token, nil
}

func (me *Peer) sendHighToPeer(to crypto.Hash, typ byte, key, data []byte) error {
	return me.sendToPeer(to, typ, key, data, MsgPriorityHigh)
}

func (me *Peer) offerToPeerWithCacheCheck(p *Peer, priority int, msg *ChanMsg) bool {
	if p.IdForNetwork == me.IdForNetwork {
		return true
	}
	if me.snapshotsCaches.contains(msg.key, time.Minute) {
		return true
	}
	return p.offer(priority, msg)
}

func (p *Peer) offer(priority int, msg *ChanMsg) bool {
	if p.closing.Load() {
		return false
	}
	var ok bool
	switch priority {
	case MsgPriorityNormal:
		select {
		case p.normalRing <- msg:
			ok = true
		default:
		}
	case MsgPriorityHigh:
		select {
		case p.highRing <- msg:
			ok = true
		default:
		}
	default:
		panic(priority)
	}
	if ok {
		p.wakeSendingStream()
	}
	return ok
}

// Wait for new messages instead of polling on a fixed cadence,
// so every message hop avoids the average half-cadence delay.
// The timer is only a fallback to re-check the closing flags.
func (p *Peer) waitSendingStream(wait time.Duration) {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-p.sendWake:
	case <-timer.C:
	}
}

// wakeSendingStream signals loopSendingStream that a new message is
// available. The channel has capacity one, so repeated signals coalesce
// into a single immediate iteration instead of queueing up.
func (p *Peer) wakeSendingStream() {
	select {
	case p.sendWake <- struct{}{}:
	default:
	}
}

func (me *Peer) sendToPeer(to crypto.Hash, typ byte, key, data []byte, priority int) error {
	if to == me.IdForNetwork {
		return nil
	}
	if me.snapshotsCaches.contains(key, time.Minute) {
		return nil
	}
	me.sentMetric.handle(typ)

	nbrs := me.GetNeighbors(to)
	for _, peer := range nbrs {
		success := peer.offer(priority, &ChanMsg{key, data})
		if success { // no double send for the same message to avoid errors
			return nil
		}
		logger.Verbosef("peer.offer(%s) send timeout\n", peer.IdForNetwork)
	}

	rm := me.buildRelayMessage(to, data)
	rk := crypto.Blake3Hash(rm)
	rk = crypto.Blake3Hash(append(rk[:], []byte("REMOTE")...))
	relayers := me.GetRemoteRelayers(to)
	if len(relayers) == 0 {
		relayers = me.relayers.Slice()
	}
	for _, peer := range relayers {
		if !peer.IsRelayer() {
			panic(peer.IdForNetwork)
		}
		rk := crypto.Blake3Hash(append(rk[:], peer.IdForNetwork[:]...))
		success := me.offerToPeerWithCacheCheck(peer, priority, &ChanMsg{rk[:], rm})
		if success {
			return nil
		}
		logger.Verbosef("me.offerToPeerWithCacheCheck(%s) send timeout\n", peer.IdForNetwork)
	}
	return nil
}

func (me *Peer) sendSnapshotMessageToPeer(to crypto.Hash, snap crypto.Hash, typ byte, data []byte) error {
	key := append(to[:], snap[:]...)
	key = append(key, 'S', 'N', 'A', 'P', typ)
	return me.sendToPeer(to, typ, key, data, MsgPriorityNormal)
}

func (me *Peer) GetNeighbors(key crypto.Hash) []*Peer {
	var nbrs []*Peer
	p := me.relayers.Get(key)
	if p != nil {
		nbrs = append(nbrs, p)
	}
	p = me.consumers.Get(key)
	if p != nil {
		nbrs = append(nbrs, p)
	}
	return nbrs
}

func (me *Peer) GetRemoteRelayers(key crypto.Hash) []*Peer {
	if me.remoteRelayers == nil {
		return nil
	}
	var relayers []*Peer
	ids := me.remoteRelayers.Get(key)
	for _, id := range ids {
		nbrs := me.GetNeighbors(id)
		relayers = append(relayers, nbrs...)
	}
	return relayers
}
