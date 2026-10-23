package rpc

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/stretchr/testify/require"
)

type readerTestStore struct {
	storage.Store
	tx       *common.VersionedTransaction
	snapshot string
	utxo     *common.UTXOWithLock
}

func (s *readerTestStore) ReadDepositLock(_ *common.DepositData) (crypto.Hash, error) {
	if s.tx == nil {
		return crypto.Hash{}, nil
	}
	return s.tx.PayloadHash(), nil
}

func (s *readerTestStore) ReadTransaction(_ crypto.Hash) (*common.VersionedTransaction, string, error) {
	return s.tx, s.snapshot, nil
}

func (s *readerTestStore) ReadUTXOLock(_ crypto.Hash, _ uint) (*common.UTXOWithLock, error) {
	return s.utxo, nil
}

type handlerTransport struct{ http.Handler }

func (h handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Result(), nil
}

func useReaderTestServer(t *testing.T, store *readerTestStore) {
	t.Helper()
	server := NewServer(&config.Custom{}, store, nil, 0)
	previous := rpcHTTPClient
	rpcHTTPClient = &http.Client{Transport: handlerTransport{server.Handler}}
	t.Cleanup(func() { rpcHTTPClient = previous })
}

func TestGetDepositTransactionBeforeAndAfterFinalization(t *testing.T) {
	account := common.NewAddressFromSeed(bytes.Repeat([]byte{7}, 64))
	tx := common.NewTransactionV5(common.BitcoinAssetId)
	tx.AddDepositInput(&common.DepositData{Chain: common.BitcoinAssetId, AssetKey: "btc", Transaction: "deposit", Amount: common.NewInteger(1)})
	tx.AddScriptOutput([]*common.Address{&account}, common.NewThresholdScript(1), common.NewInteger(1), bytes.Repeat([]byte{8}, 64))
	ver := tx.AsVersioned()
	require.NoError(t, ver.SignRaw(account.PrivateSpendKey))
	store := &readerTestStore{tx: ver}
	useReaderTestServer(t, store)

	for _, snapshot := range []string{"", crypto.Blake3Hash([]byte("snapshot")).String()} {
		store.snapshot = snapshot
		got, finalized, err := GetDepositTransaction("http://rpc/", common.BitcoinAssetId.String(), "deposit", 0)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, ver.Marshal(), got.Marshal())
		require.Equal(t, snapshot, finalized)
	}
	store.tx = nil
	got, finalized, err := GetDepositTransaction("http://rpc/", common.BitcoinAssetId.String(), "missing", 0)
	require.NoError(t, err)
	require.Nil(t, got)
	require.Empty(t, finalized)
}

func TestGetUTXOWithOptionalMask(t *testing.T) {
	store := &readerTestStore{}
	useReaderTestServer(t, store)
	hash := crypto.Blake3Hash([]byte("UTXO"))
	mask := crypto.NewKeyFromSeed(bytes.Repeat([]byte{9}, 64)).Public()
	for _, test := range []struct {
		name string
		utxo *common.UTXOWithLock
	}{
		{name: "missing"},
		{name: "node pledge", utxo: &common.UTXOWithLock{Hash: hash, Asset: common.XINAssetId, Type: common.OutputTypeNodePledge, Amount: common.KernelNodePledgeAmount}},
		{name: "script", utxo: &common.UTXOWithLock{Hash: hash, Asset: common.BitcoinAssetId, Type: common.OutputTypeScript, Amount: common.NewInteger(1), Mask: mask, Keys: []*crypto.Key{&mask}, Script: common.NewThresholdScript(1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store.utxo = test.utxo
			got, err := GetUTXO("http://rpc/", hash.String(), 0)
			require.NoError(t, err)
			require.Equal(t, test.utxo, got)
		})
	}
}
