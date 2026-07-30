package kernel

import (
	"bytes"
	"errors"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

type transactionLimitStore struct {
	storage.Store

	persistErrors []error
	persistCalls  []int
	persistForks  []bool
	ghostCalls    int
	ghostForks    []bool
	ghostError    error
	utxos         map[crypto.Hash]*common.UTXOWithLock
}

func (s *transactionLimitStore) LockAndPersistTransactions(txs []*common.VersionedTransaction, fork bool) error {
	s.persistCalls = append(s.persistCalls, len(txs))
	s.persistForks = append(s.persistForks, fork)
	if len(s.persistErrors) == 0 {
		return nil
	}
	err := s.persistErrors[0]
	s.persistErrors = s.persistErrors[1:]
	return err
}

func (s *transactionLimitStore) LockGhostKeys(_ []*crypto.Key, _ crypto.Hash, fork bool) error {
	s.ghostCalls++
	s.ghostForks = append(s.ghostForks, fork)
	return s.ghostError
}

func (s *transactionLimitStore) ReadUTXOLock(hash crypto.Hash, index uint) (*common.UTXOWithLock, error) {
	utxo := s.utxos[hash]
	if utxo == nil || utxo.Index != index {
		return nil, nil
	}
	return utxo, nil
}

func transactionLimitScript(t *testing.T, seed byte) (*common.VersionedTransaction, *common.UTXOWithLock) {
	sender := common.NewAddressFromSeed(bytes.Repeat([]byte{seed}, 64))
	recipient := common.NewAddressFromSeed(bytes.Repeat([]byte{seed + 32}, 64))

	funding := common.NewTransactionV5(common.XINAssetId)
	funding.Inputs = []*common.Input{{Genesis: []byte{seed}}}
	funding.AddScriptOutput(
		[]*common.Address{&sender},
		common.NewThresholdScript(1),
		common.NewInteger(1),
		bytes.Repeat([]byte{seed + 1}, 64),
	)
	utxo := funding.AsVersioned().UnspentOutputs()[0]

	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddInput(utxo.Hash, utxo.Index)
	tx.AddScriptOutput(
		[]*common.Address{&recipient},
		common.NewThresholdScript(1),
		common.NewInteger(1),
		bytes.Repeat([]byte{seed + 2}, 64),
	)
	ver := tx.AsVersioned()
	require.NoError(t, ver.SignUTXO(&utxo.UTXO, []*common.Address{&sender}))
	return ver, utxo
}

func TestLockAndPersistTransactionsAfterTxnTooBig(t *testing.T) {
	first, firstUTXO := transactionLimitScript(t, 1)
	second, secondUTXO := transactionLimitScript(t, 2)
	txs := []*common.VersionedTransaction{first, second}
	utxos := map[crypto.Hash]*common.UTXOWithLock{
		firstUTXO.Hash:  firstUTXO,
		secondUTXO.Hash: secondUTXO,
	}

	t.Run("retry reduced batch", func(t *testing.T) {
		store := &transactionLimitStore{
			persistErrors: []error{badger.ErrTxnTooBig, nil},
			utxos:         utxos,
		}
		node := &Node{persistStore: store}

		err := node.lockAndPersistTransactions(txs, 123, true)
		require.NoError(t, err)
		require.Equal(t, []int{2, 1, 1}, store.persistCalls)
		require.Equal(t, []bool{true, true, true}, store.persistForks)
		require.Equal(t, 2, store.ghostCalls)
		require.Equal(t, []bool{true, true}, store.ghostForks)
	})

	t.Run("fallback to single transactions", func(t *testing.T) {
		store := &transactionLimitStore{
			persistErrors: []error{badger.ErrTxnTooBig, nil, nil},
			utxos:         utxos,
		}
		node := &Node{persistStore: store}

		err := node.lockAndPersistTransactions(txs, 123, false)
		require.NoError(t, err)
		require.Equal(t, []int{2, 1, 1}, store.persistCalls)
		require.Equal(t, []bool{false, false, false}, store.persistForks)
		require.Equal(t, 2, store.ghostCalls)
	})

	t.Run("return revalidation error", func(t *testing.T) {
		revalidationError := errors.New("ghost lock failure")
		store := &transactionLimitStore{
			persistErrors: []error{badger.ErrTxnTooBig},
			ghostError:    revalidationError,
			utxos:         utxos,
		}
		node := &Node{persistStore: store}

		err := node.lockAndPersistTransactions(txs, 123, false)
		require.ErrorIs(t, err, revalidationError)
		require.Equal(t, []int{2}, store.persistCalls)
		require.Equal(t, 1, store.ghostCalls)
	})
}

func TestDetermineBestRound(t *testing.T) {
	require := require.New(t)

	root := t.TempDir()

	node := setupTestNode(require, root)
	require.NotNil(node)

	chain := node.BootChain(node.IdForNetwork)
	best := chain.determineBestRound(clock.NowUnixNano())
	require.Nil(best)

	chain = node.BootChain(node.genesisNodes[0])
	best = chain.determineBestRound(clock.NowUnixNano())
	require.NotNil(best)
}
