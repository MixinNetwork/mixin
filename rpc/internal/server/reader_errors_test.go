package server

import (
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
