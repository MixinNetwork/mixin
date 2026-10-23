package server

import (
	"strconv"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/stretchr/testify/require"
)

type missingReaderStore struct{ storage.Store }

func (missingReaderStore) ReadRound(crypto.Hash) (*common.Round, error) {
	return nil, nil
}

func (missingReaderStore) ReadSnapshot(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error) {
	return &common.SnapshotWithTopologicalOrder{Snapshot: &common.Snapshot{
		Hash: hash, Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("missing transaction"))},
	}}, nil
}

func (missingReaderStore) ReadTransaction(crypto.Hash) (*common.VersionedTransaction, string, error) {
	return nil, "", nil
}

func TestReadersReportMissingRecords(t *testing.T) {
	hash := crypto.Blake3Hash([]byte("missing round"))
	_, err := getRoundByNumber(nil, missingReaderStore{}, []any{hash.String(), 0})
	require.ErrorContains(t, err, "round not found")
	_, err = getSnapshot(nil, missingReaderStore{}, []any{hash.String()})
	require.ErrorContains(t, err, "transaction")
	require.ErrorContains(t, err, "not found")
}

type utxoIndexStore struct {
	storage.Store
	indices []uint
}

func (s *utxoIndexStore) ReadUTXOLock(_ crypto.Hash, index uint) (*common.UTXOWithLock, error) {
	s.indices = append(s.indices, index)
	return nil, nil
}

func TestGetUTXOIndexBounds(t *testing.T) {
	hash := crypto.Blake3Hash([]byte("UTXO index bounds"))
	for _, index := range []uint{0, common.InputIndexLimit, common.InputIndexLimit + 1, 65535} {
		t.Run(strconv.FormatUint(uint64(index), 10), func(t *testing.T) {
			store := &utxoIndexStore{}
			_, err := getUTXO(store, []any{hash.String(), index})
			if index > common.InputIndexLimit {
				require.ErrorContains(t, err, "invalid UTXO index")
				require.Empty(t, store.indices)
			} else {
				require.NoError(t, err)
				require.Equal(t, []uint{index}, store.indices)
			}
		})
	}
}
