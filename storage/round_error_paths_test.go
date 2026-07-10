package storage

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestRoundTransitionReadErrors(t *testing.T) {
	node := crypto.Blake3Hash([]byte("round read error node"))
	externalHash := crypto.Blake3Hash([]byte("round read error external"))
	externalNode := crypto.Blake3Hash([]byte("round read error external node"))
	selfHash := crypto.Blake3Hash([]byte("round read error self"))
	refs := &common.RoundLink{Self: selfHash, External: externalHash}
	self := &common.Round{Hash: node, NodeId: node, Number: 1, References: refs}
	external := &common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}}

	t.Run("update self", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphRoundKey(node), []byte{0})
		}))
		require.Error(t, store.UpdateEmptyHeadRound(node, 1, refs))
	})

	t.Run("update external", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, node, self); err != nil {
				return err
			}
			return txn.Set(graphRoundKey(externalHash), []byte{0})
		}))
		require.Error(t, store.UpdateEmptyHeadRound(node, 1, refs))
	})

	t.Run("update snapshots", func(t *testing.T) {
		store := newTestBadgerStore(t)
		corrupt := crypto.Blake3Hash([]byte("corrupt round snapshot"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, node, self); err != nil {
				return err
			}
			if err := writeRound(txn, externalHash, external); err != nil {
				return err
			}
			return txn.Set(graphSnapshotKey(node, 1, corrupt), []byte{0})
		}))
		require.Error(t, store.UpdateEmptyHeadRound(node, 1, refs))
		_, err := store.ReadSnapshotsForNodeRound(node, 1)
		require.Error(t, err)
	})

	t.Run("start self", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphRoundKey(node), []byte{0})
		}))
		require.Error(t, store.StartNewRound(node, 1, refs, 1))
	})

	t.Run("start external", func(t *testing.T) {
		store := newTestBadgerStore(t)
		previous := &common.Round{Hash: node, NodeId: node, Number: 0, References: &common.RoundLink{}}
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, node, previous); err != nil {
				return err
			}
			return txn.Set(graphRoundKey(externalHash), []byte{0})
		}))
		require.Error(t, store.StartNewRound(node, 1, refs, 1))
	})

	t.Run("start previous self", func(t *testing.T) {
		store := newTestBadgerStore(t)
		previous := &common.Round{Hash: node, NodeId: node, Number: 0, References: &common.RoundLink{}}
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, node, previous); err != nil {
				return err
			}
			if err := writeRound(txn, externalHash, external); err != nil {
				return err
			}
			return txn.Set(graphRoundKey(selfHash), []byte{0})
		}))
		require.Error(t, store.StartNewRound(node, 1, refs, 1))
	})

	t.Run("internal self and external", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphRoundKey(node), []byte{0}); err != nil {
				return err
			}
			require.Error(t, startNewRound(txn, node, 1, refs, 1))
			return nil
		}))

		store = newTestBadgerStore(t)
		previous := &common.Round{Hash: node, NodeId: node, Number: 0, References: &common.RoundLink{}}
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, node, previous); err != nil {
				return err
			}
			if err := txn.Set(graphRoundKey(externalHash), []byte{0}); err != nil {
				return err
			}
			require.Error(t, startNewRound(txn, node, 1, refs, 1))
			return nil
		}))
	})
}

func TestCustodianMintAndUTXOReadErrors(t *testing.T) {
	t.Run("custodian payload", func(t *testing.T) {
		store := newTestBadgerStore(t)
		badTx := common.NewTransactionV5(common.XINAssetId)
		badTx.Inputs = []*common.Input{{Genesis: []byte("bad custodian payload")}}
		badTx.Extra = []byte{1}
		bad := badTx.AsVersioned()
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphTransactionKey(bad.PayloadHash()), bad.Marshal()); err != nil {
				return err
			}
			hash := bad.PayloadHash()
			return txn.Set(graphCustodianUpdateKey(1), hash[:])
		}))
		_, err := store.ListCustodianUpdates()
		require.Error(t, err)
		_, err = store.ReadCustodian(1)
		require.Error(t, err)

		_, validExtra := buildCustodianUpdateTransaction(70, crypto.Blake3Hash([]byte("valid custodian payload")))
		err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
			utxo := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: crypto.Blake3Hash([]byte("new custodian"))}}}
			return writeCustodianNodes(txn, 2, utxo, validExtra, true)
		})
		require.Error(t, err)
	})

	t.Run("different custodian at same timestamp", func(t *testing.T) {
		store := newTestBadgerStore(t)
		ver1, extra1 := buildCustodianUpdateTransaction(70, crypto.Blake3Hash([]byte("custodian timestamp")))
		_, extra2 := buildCustodianUpdateTransaction(100, crypto.Blake3Hash([]byte("custodian timestamp")))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeTransaction(txn, ver1); err != nil {
				return err
			}
			utxo1 := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: ver1.PayloadHash()}}}
			if err := writeCustodianNodes(txn, 1, utxo1, extra1, true); err != nil {
				return err
			}
			utxo2 := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: crypto.Blake3Hash([]byte("different custodian"))}}}
			require.Panics(t, func() { _ = writeCustodianNodes(txn, 1, utxo2, extra2, true) })
			return nil
		}))
	})

	t.Run("malformed mint lock", func(t *testing.T) {
		store := newTestBadgerStore(t)
		mint := &common.MintData{Group: "UNIVERSAL", Batch: 1, Amount: common.NewInteger(1)}
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphMintKey(mint.Batch), []byte{0})
		}))
		err := store.LockMintInput(mint, crypto.Blake3Hash([]byte("mint replacement")), false)
		require.Error(t, err)
	})

	t.Run("malformed and finalized utxo", func(t *testing.T) {
		store := newTestBadgerStore(t)
		hash := crypto.Blake3Hash([]byte("malformed locked utxo"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphUtxoKey(hash, 0), []byte{0})
		}))
		err := store.LockUTXOs([]*common.Input{{Hash: hash, Index: 0}}, crypto.Blake3Hash([]byte("new lock")), false)
		require.Error(t, err)

		store = newTestBadgerStore(t)
		old := crypto.Blake3Hash([]byte("finalized utxo lock"))
		utxo := &common.UTXOWithLock{UTXO: common.UTXO{
			Input:  common.Input{Hash: hash},
			Output: common.Output{Type: common.OutputTypeScript, Amount: common.NewInteger(1)},
		}, LockHash: old}
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphUtxoKey(hash, 0), utxo.Marshal()); err != nil {
				return err
			}
			return txn.Set(graphFinalizationKey(old), old[:])
		}))
		err = store.LockUTXOs([]*common.Input{{Hash: hash, Index: 0}}, crypto.Blake3Hash([]byte("replacement lock")), true)
		require.ErrorContains(t, err, "prune finalized transaction")
	})

	t.Run("zero node timestamp", func(t *testing.T) {
		store := newTestBadgerStore(t)
		signer, payee := seededAddress(240), seededAddress(241)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			key := nodeStateQueueKey(signer.PublicSpendKey, 0)
			val := nodeEntryValue(payee.PublicSpendKey, crypto.Blake3Hash([]byte("zero timestamp")), common.NodeStateAccepted)
			return txn.Set(key, val)
		}))
		require.Panics(t, func() { store.ReadAllNodes(1, true) })
	})
}
