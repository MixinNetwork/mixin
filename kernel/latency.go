package kernel

import (
	"container/list"
	"fmt"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	// Keep tracing useful on long-running nodes without allowing either
	// unfinished transactions or raw duration samples to grow without bound.
	txLatencyPendingLimit = 1 << 16
	txLatencySampleLimit  = 1 << 16
)

// Transaction-to-snapshot latency instrumentation is enabled with
// MIXIN_TX_SNAPSHOT_TRACE=1. time.Now carries a monotonic clock reading, so
// elapsed durations are unaffected by wall-clock corrections while the kernel
// clock remains free to be mocked in tests.
var txSnapshotTrace = os.Getenv("MIXIN_TX_SNAPSHOT_TRACE") == "1"

// Duration samples are aggregated for reporting, while transaction timestamps
// remain on each Node's tracker. A normal production process has one Node, so
// the summary describes that node's first-cache-to-local-persistence latency.
var txLatencyMetrics = newTxLatencyRecorder(txLatencySampleLimit)

// txLatencyTracker records one node's view of a transaction. Keeping this state
// per node is important in production: gossip arrival, proposal preparation,
// and local persistence are all node-local events.
type txLatencyTracker struct {
	mu sync.Mutex

	recorder     *txLatencyRecorder
	generation   uint64
	pendingLimit int
	pending      map[crypto.Hash]*txLatencyPending
	pendingOrder list.List

	completed      map[crypto.Hash]*list.Element
	completedOrder list.List
}

type txLatencyPending struct {
	seen      time.Time
	announced time.Time
	order     *list.Element
}

func newTxLatencyTracker() *txLatencyTracker {
	return newTxLatencyTrackerForRecorder(txLatencyMetrics, txLatencyPendingLimit)
}

func newTxLatencyTrackerForRecorder(recorder *txLatencyRecorder, pendingLimit int) *txLatencyTracker {
	return &txLatencyTracker{
		recorder:     recorder,
		generation:   recorder.currentGeneration(),
		pendingLimit: max(pendingLimit, 1),
		pending:      make(map[crypto.Hash]*txLatencyPending),
		completed:    make(map[crypto.Hash]*list.Element),
	}
}

// mark records the first successful cache of a transaction on this node.
func (t *txLatencyTracker) mark(hash crypto.Hash) {
	if txSnapshotTrace {
		t.markAt(hash, time.Now())
	}
}

func (t *txLatencyTracker) markAt(hash crypto.Hash, at time.Time) {
	generation := t.recorder.currentGeneration()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.syncGenerationLocked(generation)

	if _, ok := t.pending[hash]; ok {
		return
	}
	if _, ok := t.completed[hash]; ok {
		return
	}
	if len(t.pending) >= t.pendingLimit {
		oldest := t.pendingOrder.Front()
		delete(t.pending, oldest.Value.(crypto.Hash))
		t.pendingOrder.Remove(oldest)
	}
	order := t.pendingOrder.PushBack(hash)
	t.pending[hash] = &txLatencyPending{seen: at, order: order}
}

// markAnnounced records when this node first prepares a proposal containing
// the transaction. It intentionally precedes peer transmission, so the
// following stage includes announcement delivery and proposal retries.
func (t *txLatencyTracker) markAnnounced(hash crypto.Hash) {
	if txSnapshotTrace {
		t.markAnnouncedAt(hash, time.Now())
	}
}

func (t *txLatencyTracker) markAnnouncedAt(hash crypto.Hash, at time.Time) {
	generation := t.recorder.currentGeneration()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.syncGenerationLocked(generation)

	pending := t.pending[hash]
	if pending != nil && pending.announced.IsZero() {
		pending.announced = at
	}
}

// observe records when this node successfully persists a snapshot containing
// the transaction.
func (t *txLatencyTracker) observe(hash crypto.Hash) {
	if txSnapshotTrace {
		t.observeAt(hash, time.Now())
	}
}

func (t *txLatencyTracker) observeAt(hash crypto.Hash, at time.Time) {
	generation := t.recorder.currentGeneration()
	t.mu.Lock()
	t.syncGenerationLocked(generation)

	pending := t.pending[hash]
	if pending == nil {
		t.rememberCompletedLocked(hash)
		t.mu.Unlock()
		return
	}
	delete(t.pending, hash)
	t.pendingOrder.Remove(pending.order)
	t.rememberCompletedLocked(hash)
	snapshot := at.Sub(pending.seen)
	announced := !pending.announced.IsZero()
	var queue, consensus time.Duration
	if announced {
		queue = pending.announced.Sub(pending.seen)
		consensus = at.Sub(pending.announced)
	}
	t.mu.Unlock()

	t.recorder.add(generation, snapshot, queue, consensus, announced)
}

func (t *txLatencyTracker) syncGenerationLocked(generation uint64) {
	if t.generation == generation {
		return
	}
	t.generation = generation
	clear(t.pending)
	t.pendingOrder.Init()
	clear(t.completed)
	t.completedOrder.Init()
}

func (t *txLatencyTracker) rememberCompletedLocked(hash crypto.Hash) {
	if _, ok := t.completed[hash]; ok {
		return
	}
	if len(t.completed) >= t.pendingLimit {
		oldest := t.completedOrder.Front()
		delete(t.completed, oldest.Value.(crypto.Hash))
		t.completedOrder.Remove(oldest)
	}
	t.completed[hash] = t.completedOrder.PushBack(hash)
}

// txLatencyStat retains a bounded window of the most recent observations.
// Access is protected by the owning txLatencyRecorder mutex.
type txLatencyStat struct {
	limit    int
	samples  []time.Duration
	next     int
	observed uint64
}

func newTxLatencyStat(limit int) txLatencyStat {
	return txLatencyStat{limit: max(limit, 1)}
}

func (s *txLatencyStat) add(d time.Duration) {
	s.observed++
	if len(s.samples) < s.limit {
		s.samples = append(s.samples, d)
		return
	}
	s.samples[s.next] = d
	s.next = (s.next + 1) % s.limit
}

func (s *txLatencyStat) reset() {
	s.samples = s.samples[:0]
	s.next = 0
	s.observed = 0
}

// txLatencyRecorder aggregates bounded duration samples from Node-local
// trackers. generation allows ResetTxSnapshotLatency to define an atomic batch
// boundary without keeping a process-wide registry of nodes.
type txLatencyRecorder struct {
	mu         sync.Mutex
	generation atomic.Uint64
	snapshot   txLatencyStat
	queue      txLatencyStat
	consensus  txLatencyStat
}

func newTxLatencyRecorder(sampleLimit int) *txLatencyRecorder {
	return &txLatencyRecorder{
		snapshot:  newTxLatencyStat(sampleLimit),
		queue:     newTxLatencyStat(sampleLimit),
		consensus: newTxLatencyStat(sampleLimit),
	}
}

func (r *txLatencyRecorder) currentGeneration() uint64 {
	return r.generation.Load()
}

func (r *txLatencyRecorder) add(
	generation uint64,
	snapshot, queue, consensus time.Duration,
	announced bool,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation != r.generation.Load() {
		return
	}

	r.snapshot.add(snapshot)
	if announced {
		r.queue.add(queue)
		r.consensus.add(consensus)
	}
}

func (r *txLatencyRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.generation.Add(1)
	r.snapshot.reset()
	r.queue.reset()
	r.consensus.reset()
}

type txLatencyStatSnapshot struct {
	samples  []time.Duration
	observed uint64
}

func (r *txLatencyRecorder) summaries() (txLatencyStatSnapshot, txLatencyStatSnapshot, txLatencyStatSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyStat := func(stat *txLatencyStat) txLatencyStatSnapshot {
		return txLatencyStatSnapshot{
			samples:  append([]time.Duration(nil), stat.samples...),
			observed: stat.observed,
		}
	}
	return copyStat(&r.snapshot), copyStat(&r.queue), copyStat(&r.consensus)
}

// ResetTxSnapshotLatency starts a new measurement generation. Existing
// per-node timestamps become ineligible immediately and are cleared lazily on
// the next tracker event, so an earlier batch cannot contaminate the new one.
func ResetTxSnapshotLatency() {
	txLatencyMetrics.reset()
}

func (s txLatencyStatSnapshot) summary(label string) string {
	if len(s.samples) == 0 {
		return fmt.Sprintf("%s: no samples", label)
	}
	slices.Sort(s.samples)
	var sum time.Duration
	for _, d := range s.samples {
		sum += d
	}
	pct := func(p int) time.Duration {
		idx := (len(s.samples)*p + 99) / 100
		idx = max(idx, 1)
		return s.samples[idx-1]
	}
	count := fmt.Sprintf("samples=%d", len(s.samples))
	if s.observed > uint64(len(s.samples)) {
		count += fmt.Sprintf("/%d(latest)", s.observed)
	}
	return fmt.Sprintf("%s %s min=%s p50=%s p90=%s p99=%s max=%s avg=%s",
		label, count, s.samples[0], pct(50), pct(90), pct(99),
		s.samples[len(s.samples)-1], sum/time.Duration(len(s.samples)))
}

// TxSnapshotLatencySummary reports Node-local observations. A production
// process normally has one Node. A multi-node in-process test intentionally
// contributes one local persistence sample from each node that saw the
// transaction; use the client observer for a unique end-to-end test metric.
func TxSnapshotLatencySummary() string {
	snapshot, queue, consensus := txLatencyMetrics.summaries()
	return snapshot.summary("local first-cache->local-persist") +
		" | " + queue.summary("local first-cache->first-proposal") +
		" | " + consensus.summary("local first-proposal->local-persist")
}
