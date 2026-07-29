package kernel

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

// Transaction-to-snapshot latency instrumentation, enabled with
// MIXIN_TX_SNAPSHOT_TRACE=1. All timestamps use the real wall clock on
// purpose, because the kernel clock is mocked in tests.
var txSnapshotTrace = os.Getenv("MIXIN_TX_SNAPSHOT_TRACE") == "1"

// txLatencyTracker records the first moment this node sees a transaction,
// the moment this node announces a snapshot proposal containing it (only on
// the proposing node), and measures the delay until this node writes a
// snapshot containing it.
type txLatencyTracker struct {
	mu        sync.Mutex
	seen      map[crypto.Hash]time.Time
	announced map[crypto.Hash]time.Time
}

func newTxLatencyTracker() *txLatencyTracker {
	return &txLatencyTracker{
		seen:      make(map[crypto.Hash]time.Time),
		announced: make(map[crypto.Hash]time.Time),
	}
}

// mark records the first moment this node sees a transaction, either from an
// RPC submission or from a peer gossip message.
func (t *txLatencyTracker) mark(hash crypto.Hash) {
	if !txSnapshotTrace {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.seen[hash]; ok {
		return
	}
	if len(t.seen) >= 1<<20 {
		return
	}
	t.seen[hash] = time.Now()
}

// markAnnounced records the moment this node, acting as the snapshot
// proposer, sends the CoSi announcement for a snapshot containing the
// transaction.
func (t *txLatencyTracker) markAnnounced(hash crypto.Hash) {
	if !txSnapshotTrace {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.seen[hash]; !ok {
		return
	}
	if _, ok := t.announced[hash]; !ok {
		t.announced[hash] = time.Now()
	}
}

// observe measures the latency between the first mark and the moment a
// snapshot containing the transaction is written by this node. On the
// proposing node it also splits the latency into queueing (seen -> announced)
// and consensus (announced -> finalized) stages. Only the first observation
// per transaction is kept.
func (t *txLatencyTracker) observe(hash crypto.Hash) {
	if !txSnapshotTrace {
		return
	}
	t.mu.Lock()
	at, ok := t.seen[hash]
	if ok {
		delete(t.seen, hash)
	}
	ann, announced := t.announced[hash]
	delete(t.announced, hash)
	t.mu.Unlock()
	if !ok {
		return
	}
	now := time.Now()
	txSnapshotLatencies.add(now.Sub(at))
	if announced {
		txQueueLatencies.add(ann.Sub(at))
		txConsensusLatencies.add(now.Sub(ann))
	}
}

type txLatencyStat struct {
	mu      sync.Mutex
	samples []time.Duration
}

var (
	txSnapshotLatencies  = &txLatencyStat{}
	txQueueLatencies     = &txLatencyStat{}
	txConsensusLatencies = &txLatencyStat{}
)

func (s *txLatencyStat) add(d time.Duration) {
	s.mu.Lock()
	s.samples = append(s.samples, d)
	s.mu.Unlock()
}

func (s *txLatencyStat) reset() {
	s.mu.Lock()
	s.samples = nil
	s.mu.Unlock()
}

// ResetTxSnapshotLatency clears all collected samples, so a test can measure
// one batch of transactions at a time.
func ResetTxSnapshotLatency() {
	txSnapshotLatencies.reset()
	txQueueLatencies.reset()
	txConsensusLatencies.reset()
}

func (s *txLatencyStat) summary(label string) string {
	s.mu.Lock()
	ds := append([]time.Duration(nil), s.samples...)
	s.mu.Unlock()
	if len(ds) == 0 {
		return fmt.Sprintf("%s: no samples", label)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	pct := func(p float64) time.Duration {
		idx := int(float64(len(ds))*p+0.5) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(ds) {
			idx = len(ds) - 1
		}
		return ds[idx]
	}
	return fmt.Sprintf("%s samples=%d min=%s p50=%s p90=%s p99=%s max=%s avg=%s",
		label, len(ds), ds[0], pct(0.5), pct(0.9), pct(0.99), ds[len(ds)-1], sum/time.Duration(len(ds)))
}

// TxSnapshotLatencySummary returns count/min/p50/p90/p99/max/avg of the
// observed transaction-to-snapshot latencies across all in-process nodes.
// The queue and consensus stages are only sampled on proposing nodes.
func TxSnapshotLatencySummary() string {
	return txSnapshotLatencies.summary("tx->snapshot latency") +
		" | " + txQueueLatencies.summary("queue") +
		" | " + txConsensusLatencies.summary("consensus")
}
