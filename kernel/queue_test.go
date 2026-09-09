package kernel

import (
	"bytes"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/stretchr/testify/require"
)

func TestQueueTransactionRejectedDepositDoesNotLockGhostKeys(t *testing.T) {
	require := require.New(t)
	dir := t.TempDir()
	custom := &config.Custom{}
	custom.Node.CacheTTL = 3600
	store, err := storage.NewBadgerStore(custom, dir)
	require.NoError(err)
	t.Cleanup(func() {
		if store != nil {
			require.NoError(store.Close())
		}
	})
	node := &Node{persistStore: store}

	account := common.NewAddressFromSeed(bytes.Repeat([]byte{91}, 64))
	tx := common.NewTransactionV5(common.BitcoinAssetId)
	tx.AddDepositInput(&common.DepositData{
		Chain:       common.BitcoinAssetId,
		AssetKey:    "bitcoin",
		Transaction: "rejected-deposit",
		Amount:      common.NewInteger(1),
	})
	tx.AddScriptOutput([]*common.Address{&account}, common.NewThresholdScript(1), common.NewInteger(1), bytes.Repeat([]byte{92}, 64))
	ver := tx.AsVersioned()
	ver.SignaturesMap = []map[uint16]*crypto.Signature{{}}
	ver, err = common.UnmarshalVersionedTransaction(ver.Marshal())
	require.NoError(err)
	key := *ver.Outputs[0].Keys[0]

	_, err = node.QueueTransaction(ver)
	require.ErrorContains(err, "invalid signatures count")
	lock, err := store.ReadGhostKeyLock(key)
	require.NoError(err)
	require.Nil(lock)

	require.NoError(store.Close())
	store, err = storage.NewBadgerStore(custom, dir)
	require.NoError(err)
	lock, err = store.ReadGhostKeyLock(key)
	require.NoError(err)
	require.Nil(lock)

	// A rejected submission must leave the key available to another transaction.
	owner := crypto.Blake3Hash([]byte("different transaction"))
	require.NoError(store.LockGhostKeys([]*crypto.Key{&key}, owner, false))
	lock, err = store.ReadGhostKeyLock(key)
	require.NoError(err)
	require.NotNil(lock)
	require.Equal(owner, *lock)
}
