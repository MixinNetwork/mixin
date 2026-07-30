package kernel

import (
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestTxLatencyTrackerPreservesNodeLocalSamples(t *testing.T) {
	require := require.New(t)
	recorder := newTxLatencyRecorder(8)
	firstNode := newTxLatencyTrackerForRecorder(recorder, 8)
	secondNode := newTxLatencyTrackerForRecorder(recorder, 8)
	hash := crypto.Blake3Hash([]byte("node-local transaction"))
	start := time.Unix(1, 0)

	firstNode.markAt(hash, start)
	firstNode.markAt(hash, start.Add(5*time.Millisecond))
	firstNode.markAnnouncedAt(hash, start.Add(10*time.Millisecond))
	firstNode.markAnnouncedAt(hash, start.Add(15*time.Millisecond))
	secondNode.markAt(hash, start.Add(5*time.Millisecond))

	firstNode.observeAt(hash, start.Add(30*time.Millisecond))
	secondNode.observeAt(hash, start.Add(40*time.Millisecond))

	// Delayed duplicate events on either node must not create more samples.
	firstNode.markAt(hash, start.Add(50*time.Millisecond))
	firstNode.observeAt(hash, start.Add(60*time.Millisecond))

	snapshot, queue, consensus := recorder.summaries()
	require.ElementsMatch([]time.Duration{30 * time.Millisecond, 35 * time.Millisecond}, snapshot.samples)
	require.Equal([]time.Duration{10 * time.Millisecond}, queue.samples)
	require.Equal([]time.Duration{20 * time.Millisecond}, consensus.samples)
	require.Equal(uint64(2), snapshot.observed)
}

func TestTxLatencyTrackerResetIsAtomicBatchBoundary(t *testing.T) {
	require := require.New(t)
	recorder := newTxLatencyRecorder(8)
	tracker := newTxLatencyTrackerForRecorder(recorder, 8)
	start := time.Unix(1, 0)
	oldHash := crypto.Blake3Hash([]byte("old batch"))
	newHash := crypto.Blake3Hash([]byte("new batch"))

	tracker.markAt(oldHash, start)
	tracker.markAnnouncedAt(oldHash, start.Add(time.Millisecond))
	recorder.reset()
	tracker.observeAt(oldHash, start.Add(2*time.Millisecond))

	tracker.markAt(newHash, start.Add(3*time.Millisecond))
	tracker.observeAt(newHash, start.Add(7*time.Millisecond))

	snapshot, queue, consensus := recorder.summaries()
	require.Equal([]time.Duration{4 * time.Millisecond}, snapshot.samples)
	require.Empty(queue.samples)
	require.Empty(consensus.samples)
	require.Equal(uint64(1), snapshot.observed)
}

func TestTxLatencyTrackerBoundsPendingAndSamples(t *testing.T) {
	require := require.New(t)
	recorder := newTxLatencyRecorder(2)
	tracker := newTxLatencyTrackerForRecorder(recorder, 2)
	start := time.Unix(1, 0)
	hashes := []crypto.Hash{
		crypto.Blake3Hash([]byte("one")),
		crypto.Blake3Hash([]byte("two")),
		crypto.Blake3Hash([]byte("three")),
	}

	for i, hash := range hashes {
		tracker.markAt(hash, start.Add(time.Duration(i)*time.Millisecond))
	}
	require.Len(tracker.pending, 2)

	// The oldest pending entry was evicted, while later observations remain
	// usable and tracing continues instead of stopping at the capacity limit.
	tracker.observeAt(hashes[0], start.Add(10*time.Millisecond))
	tracker.observeAt(hashes[1], start.Add(11*time.Millisecond))
	tracker.observeAt(hashes[2], start.Add(12*time.Millisecond))
	snapshot, _, _ := recorder.summaries()
	require.Len(snapshot.samples, 2)
	require.Equal(uint64(2), snapshot.observed)
	require.Empty(tracker.pending)

	recorder.reset()
	for i, hash := range hashes {
		at := start.Add(time.Duration(i) * time.Second)
		tracker.markAt(hash, at)
		tracker.observeAt(hash, at.Add(time.Duration(i+1)*time.Millisecond))
	}
	snapshot, _, _ = recorder.summaries()
	require.Equal(uint64(3), snapshot.observed)
	require.ElementsMatch([]time.Duration{2 * time.Millisecond, 3 * time.Millisecond}, snapshot.samples)
}
