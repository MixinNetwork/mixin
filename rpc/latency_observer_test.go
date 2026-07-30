package rpc

import (
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestTxLatencyObserverBatchLifecycle(t *testing.T) {
	require := require.New(t)
	observer := newTxLatencyObserver()
	defer observer.close()

	ver := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	hash := ver.PayloadHash()
	first := time.Unix(1, 0)

	observer.markSend(hash, first)
	_, ok := observer.sendTimes.Load(hash)
	require.False(ok, "untracked traffic must not create measurement state")

	observer.track([]*common.VersionedTransaction{ver})
	observer.markSend(hash, first)
	observer.markSend(hash, first.Add(time.Second))
	sent, ok := observer.sendTimes.Load(hash)
	require.True(ok)
	require.Equal(first, sent)

	require.True(observer.claim(hash))
	require.False(observer.claim(hash), "a retry must not start a second poller")

	observer.latencies.Store(hash, 5*time.Millisecond)
	require.Equal(1, observer.waitFor([]crypto.Hash{hash}, time.Second))

	observer.recordMaxSweep(2 * time.Millisecond)
	observer.recordMaxSweep(time.Millisecond)
	observer.recordMaxSweep(3 * time.Millisecond)
	require.Equal(3*time.Millisecond, time.Duration(observer.maxSweep.Load()))

	observer.finish([]*common.VersionedTransaction{ver})
	_, ok = observer.sendTimes.Load(hash)
	require.False(ok)
	_, ok = observer.latencies.Load(hash)
	require.False(ok)
	_, ok = observer.tracked.Load(hash)
	require.False(ok)
	_, ok = observer.observing.Load(hash)
	require.False(ok)
}
