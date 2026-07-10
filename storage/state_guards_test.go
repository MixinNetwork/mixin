package storage

import (
	"sort"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestAssetAndSpaceStateGuards(t *testing.T) {
	t.Run("asset capacity and identity", func(t *testing.T) {
		store := newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			asset := &common.Asset{Chain: common.BitcoinAssetId, AssetKey: "btc"}
			require.NoError(t, writeAssetInfo(txn, common.BitcoinAssetId, asset))
			require.Error(t, writeAssetInfo(txn, common.BitcoinAssetId, &common.Asset{
				Chain:    common.BitcoinAssetId,
				AssetKey: "different",
			}))

			tx := common.NewTransactionV5(common.BitcoinAssetId)
			tx.Inputs = []*common.Input{{Genesis: []byte("capacity")}}
			tx.Outputs = []*common.Output{{
				Type:   common.OutputTypeScript,
				Amount: common.NewInteger(2501),
			}}
			require.Panics(t, func() {
				_ = writeTotalInAsset(txn, tx.AsVersioned())
			})
			return nil
		})
		require.NoError(t, err)
	})

	tests := []struct {
		name  string
		space *common.RoundSpace
		seed  *common.RoundSpace
	}{
		{
			name:  "checkpoint rollback",
			seed:  &common.RoundSpace{Batch: 2, Round: 2},
			space: &common.RoundSpace{Batch: 1, Round: 2},
		},
		{
			name: "duration on round zero",
			space: &common.RoundSpace{
				Batch: 1, Round: 0, Duration: uint64(config.CheckpointDuration),
			},
		},
		{
			name: "duration below checkpoint",
			space: &common.RoundSpace{
				Batch: 1, Round: 1, Duration: uint64(config.CheckpointDuration) - 1,
			},
		},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newTestBadgerStore(t)
			node := crypto.Blake3Hash([]byte{byte(i + 1)})
			if test.seed != nil {
				seed := *test.seed
				seed.NodeId = node
				require.NoError(t, store.WriteRoundSpaceAndState(&seed))
			}
			space := *test.space
			space.NodeId = node
			require.Panics(t, func() {
				_ = store.WriteRoundSpaceAndState(&space)
			})
		})
	}
}

func TestRoundWorkStateGuards(t *testing.T) {
	node := crypto.Blake3Hash([]byte("work guard node"))
	other := crypto.Blake3Hash([]byte("work guard signer"))
	hash1 := crypto.Blake3Hash([]byte("work guard one"))
	hash2 := crypto.Blake3Hash([]byte("work guard two"))

	t.Run("non consecutive round", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.Panics(t, func() {
			_ = store.WriteRoundWork(node, 2, nil, false)
		})
	})

	t.Run("same round additions and omissions", func(t *testing.T) {
		store := newTestBadgerStore(t)
		first := &common.SnapshotWork{Hash: hash1, Timestamp: DAY_U64, Signers: []crypto.Hash{node}}
		second := &common.SnapshotWork{Hash: hash2, Timestamp: DAY_U64 + 1, Signers: []crypto.Hash{node}}
		require.NoError(t, store.WriteRoundWork(node, 1, []*common.SnapshotWork{first}, false))
		require.NoError(t, store.WriteRoundWork(node, 1, []*common.SnapshotWork{first, second}, false))
		require.Panics(t, func() {
			_ = store.WriteRoundWork(node, 1, []*common.SnapshotWork{first}, false)
		})
	})

	tests := []struct {
		name  string
		works []*common.SnapshotWork
	}{
		{
			name:  "zero timestamp",
			works: []*common.SnapshotWork{{Hash: hash1, Signers: []crypto.Hash{node}}},
		},
		{
			name: "multiple days",
			works: []*common.SnapshotWork{
				{Hash: hash1, Timestamp: DAY_U64, Signers: []crypto.Hash{node}},
				{Hash: hash2, Timestamp: DAY_U64 * 2, Signers: []crypto.Hash{node}},
			},
		},
		{
			name:  "zero snapshot hash",
			works: []*common.SnapshotWork{{Timestamp: DAY_U64, Signers: []crypto.Hash{node}}},
		},
		{
			name:  "leader did not sign",
			works: []*common.SnapshotWork{{Hash: hash1, Timestamp: DAY_U64, Signers: []crypto.Hash{other}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newTestBadgerStore(t)
			require.Panics(t, func() {
				_ = store.WriteRoundWork(node, 1, test.works, true)
			})
		})
	}
}

func TestNodeTransitionRejections(t *testing.T) {
	signer1, payee1 := seededAddress(1), seededAddress(2)
	signer2, payee2 := seededAddress(3), seededAddress(4)
	hash := func(label string) crypto.Hash { return crypto.Blake3Hash([]byte(label)) }

	t.Run("cancel", func(t *testing.T) {
		store := newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodeAccept(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("accepted"), 1, true))
			require.Error(t, writeNodeCancel(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("cancel"), 2))
			return nil
		})
		require.NoError(t, err)

		store = newTestBadgerStore(t)
		err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodePledge(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("pledge"), 1))
			require.Error(t, writeNodeCancel(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, hash("cancel mismatch"), 2))
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("accept", func(t *testing.T) {
		store := newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodeAccept(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("accepted"), 1, true))
			require.Error(t, writeNodeAccept(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("accept twice"), 2, false))
			return nil
		})
		require.NoError(t, err)

		store = newTestBadgerStore(t)
		err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodePledge(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("pledge"), 1))
			require.Error(t, writeNodeAccept(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, hash("accept mismatch"), 2, false))
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("remove", func(t *testing.T) {
		store := newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodePledge(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("pledge"), 1))
			require.Error(t, writeNodeRemove(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("remove pledging"), 2))
			return nil
		})
		require.NoError(t, err)

		store = newTestBadgerStore(t)
		err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodeAccept(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("accepted"), 1, true))
			require.Error(t, writeNodeRemove(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, hash("missing"), 2))
			require.Error(t, writeNodeRemove(txn, signer1.PublicSpendKey, payee2.PublicSpendKey, hash("wrong payee"), 2))
			require.NoError(t, writeNodeRemove(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("removed"), 2))
			require.NoError(t, writeNodeAccept(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, hash("other accepted"), 3, true))
			require.Error(t, writeNodeRemove(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("remove twice"), 4))
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("pledge", func(t *testing.T) {
		store := newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			require.NoError(t, writeNodePledge(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, hash("pledge"), 1))
			require.Error(t, writeNodePledge(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, hash("other pledge"), 2))
			return nil
		})
		require.NoError(t, err)

		store = newTestBadgerStore(t)
		err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
			original := hash("accepted")
			require.NoError(t, writeNodeAccept(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, original, 1, true))
			require.Error(t, writeNodePledge(txn, signer1.PublicSpendKey, payee2.PublicSpendKey, hash("duplicate signer"), 2))
			require.Error(t, writeNodePledge(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, original, 2))
			return nil
		})
		require.NoError(t, err)
	})
}

func TestMintRoundAndCustodianCorruption(t *testing.T) {
	t.Run("round encoding", func(t *testing.T) {
		id := crypto.Blake3Hash([]byte("malformed round"))
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphRoundKey(id), []byte{0})
		}))
		_, err := store.ReadRound(id)
		require.Error(t, err)

		store = newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			round := &common.Round{NodeId: id, References: &common.RoundLink{}}
			return txn.Set(graphRoundKey(id), round.Marshal())
		}))
		require.Panics(t, func() {
			_, _ = store.ReadRound(id)
		})
	})

	t.Run("mint records", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphMintKey(1), []byte{0})
		}))
		_, _, err := store.ReadMintDistributions(0, 10)
		require.Error(t, err)
		_, err = store.ReadLastMintDistribution(10)
		require.Error(t, err)

		store = newTestBadgerStore(t)
		dist := (&common.MintData{Group: "UNIVERSAL", Batch: 2, Amount: common.NewInteger(1)}).Distribute(crypto.Blake3Hash([]byte("mint")))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphMintKey(1), dist.Marshal())
		}))
		require.Panics(t, func() { _, _, _ = store.ReadMintDistributions(0, 10) })
		require.Panics(t, func() { _, _ = store.ReadLastMintDistribution(10) })

		store = newTestBadgerStore(t)
		mint := &common.MintData{Group: "UNIVERSAL", Batch: 3, Amount: common.NewInteger(1)}
		old := crypto.Blake3Hash([]byte("finalized mint lock"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeMintDistribution(txn, mint, old); err != nil {
				return err
			}
			return txn.Set(graphFinalizationKey(old), old[:])
		}))
		err = store.LockMintInput(mint, crypto.Blake3Hash([]byte("replacement")), true)
		require.ErrorContains(t, err, "prune finalized transaction")
	})

	t.Run("custodian records", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphCustodianUpdateKey(1), []byte{1})
		}))
		require.Panics(t, func() { _, _ = store.ListCustodianUpdates() })

		store = newTestBadgerStore(t)
		hash := crypto.Blake3Hash([]byte("corrupt custodian transaction"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphTransactionKey(hash), []byte{0}); err != nil {
				return err
			}
			return txn.Set(graphCustodianUpdateKey(1), hash[:])
		}))
		_, err := store.ListCustodianUpdates()
		require.Error(t, err)

		store = newTestBadgerStore(t)
		_, extra1 := buildCustodianUpdateTransaction(70, crypto.Blake3Hash([]byte("custodian guards")))
		_, extra2 := buildCustodianUpdateTransaction(100, crypto.Blake3Hash([]byte("custodian guards")))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			utxo1 := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: crypto.Blake3Hash([]byte("custodian one"))}}}
			require.NoError(t, writeCustodianNodes(txn, 1, utxo1, extra1, true))
			utxo2 := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: crypto.Blake3Hash([]byte("custodian two"))}}}
			require.Panics(t, func() { _ = writeCustodianNodes(txn, 1, utxo2, extra2, true) })
			require.Panics(t, func() { _ = writeCustodianNodes(txn, 2, utxo2, []byte{1}, true) })
			return nil
		}))

		extra := make([]byte, 64)
		nodes := make([][]byte, 51)
		for i := range nodes {
			node := make([]byte, 353)
			node[0] = 1
			for field := 0; field < 4; field++ {
				for j := 0; j < 32; j++ {
					node[1+field*32+j] = byte(i*4 + field + 1)
				}
			}
			nodes[i] = node
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i][1] < nodes[j][1] })
		for _, node := range nodes {
			extra = append(extra, node...)
		}
		extra = append(extra, make([]byte, 64)...)
		store = newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
			utxo := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: crypto.Blake3Hash([]byte("too many custodians"))}}}
			require.Panics(t, func() { _ = writeCustodianNodes(txn, 1, utxo, extra, true) })
			return nil
		}))
	})
}
