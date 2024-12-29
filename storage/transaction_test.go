package storage

import (
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestTransaction(t *testing.T) {
	require := require.New(t)

	custom, err := config.Initialize("../config/config.example.toml")
	require.Nil(err)

	root, err := os.MkdirTemp("", "mixin-badger-test")
	require.Nil(err)
	defer os.RemoveAll(root)

	store, _ := NewBadgerStore(custom, root)
	defer store.Close()

	last, _ := store.LastSnapshot()
	require.Nil(last)

	gns, err := common.ReadGenesis("../config/genesis.json")
	require.Nil(err)
	rounds, snapshots, transactions, err := gns.BuildSnapshots()
	require.Nil(err)
	loaded, err := store.CheckGenesisLoad(snapshots)
	require.Nil(err)
	require.False(loaded)
	err = store.LoadGenesis(rounds, snapshots, transactions)
	require.Nil(err)
	loaded, err = store.CheckGenesisLoad(snapshots)
	require.Nil(err)
	require.True(loaded)
	signers := []crypto.Hash{rounds[0].NodeId}

	genesis := common.NewInteger(13439).Mul(27).Add(common.NewInteger(2700))
	_, balance, err := store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal(genesis.String(), balance.String())
	require.Equal("365553.00000000", balance.String())

	seed := make([]byte, 64)
	crypto.ReadRand(seed)
	mixin := common.NewAddressFromSeed(seed)

	deposit := common.NewTransactionV5(common.XINAssetId)
	deposit.AddDepositInput(&common.DepositData{
		Chain:       common.EthereumAssetId,
		AssetKey:    "0xMIXINASSETKEY",
		Transaction: "0xMIXINTODAMOONTRANSACTION0",
		Index:       0,
		Amount:      common.NewInteger(10),
	})
	deposit.AddScriptOutput([]*common.Address{&mixin}, common.NewThresholdScript(1), common.NewInteger(10), seed)
	deposit.AddScriptOutput([]*common.Address{&mixin}, common.NewThresholdScript(1), common.NewInteger(10), seed)

	err = store.LockDepositInput(deposit.Inputs[0].Deposit, deposit.AsVersioned().PayloadHash(), false)
	require.Nil(err)
	err = store.WriteTransaction(deposit.AsVersioned())
	require.Contains(err.Error(), "invalid asset info")

	asset, balance, err := store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal("365553.00000000", balance.String())
	deposit.Inputs[0].Deposit.AssetKey = asset.AssetKey
	deposit.Inputs[0].Deposit.Transaction = "0xMIXINTODAMOONTRANSACTION1"
	err = store.LockDepositInput(deposit.Inputs[0].Deposit, deposit.AsVersioned().PayloadHash(), false)
	require.Nil(err)
	err = store.WriteTransaction(deposit.AsVersioned())
	require.Nil(err)
	utxo, err := store.ReadUTXOLock(deposit.AsVersioned().PayloadHash(), 0)
	require.Nil(err)
	require.Nil(utxo)
	_, balance, err = store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal("365553.00000000", balance.String())

	round, _ := store.ReadRound(rounds[0].NodeId)
	require.Equal(uint64(1), round.Number)
	snap := &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       signers[0],
		RoundNumber:  1,
		Timestamp:    uint64(time.Now().UnixNano()),
		Transactions: []crypto.Hash{deposit.AsVersioned().PayloadHash()},
		References:   round.References,
	}
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         snap,
		TopologicalOrder: uint64(len(snapshots)),
	}
	err = store.WriteSnapshot(topo, signers)
	require.Nil(err)
	utxo, err = store.ReadUTXOLock(deposit.AsVersioned().PayloadHash(), 0)
	require.Nil(err)
	require.NotNil(utxo)
	_, balance, err = store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal("365563.00000000", balance.String())

	submit := common.NewTransactionV5(common.XINAssetId)
	submit.AddInput(deposit.AsVersioned().PayloadHash(), 0)
	submit.Outputs = []*common.Output{{
		Type:   common.OutputTypeWithdrawalSubmit,
		Amount: common.NewInteger(1),
		Withdrawal: &common.WithdrawalData{
			Address: "0xMIXINTODAMOON",
			Tag:     "21BTC",
		},
	}}
	err = store.LockUTXOs(submit.Inputs, submit.AsVersioned().PayloadHash(), false)
	require.Nil(err)
	err = store.WriteTransaction(submit.AsVersioned())
	require.Nil(err)
	_, balance, err = store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal("365563.00000000", balance.String())

	snap = &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       signers[0],
		RoundNumber:  1,
		Timestamp:    uint64(time.Now().UnixNano()),
		Transactions: []crypto.Hash{submit.AsVersioned().PayloadHash()},
		References:   round.References,
	}
	topo = &common.SnapshotWithTopologicalOrder{
		Snapshot:         snap,
		TopologicalOrder: uint64(len(snapshots)) + 1,
	}
	err = store.WriteSnapshot(topo, signers)
	require.Nil(err)
	_, balance, err = store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal("365562.00000000", balance.String())

	ver, ss, err := store.ReadWithdrawalClaim(submit.AsVersioned().PayloadHash())
	require.Nil(err)
	require.Equal("", ss)
	require.Nil(ver)

	claim := common.NewTransactionV5(common.XINAssetId)
	claim.AddInput(deposit.AsVersioned().PayloadHash(), 1)
	claim.Outputs = []*common.Output{{
		Type:   common.OutputTypeWithdrawalClaim,
		Amount: common.NewInteger(1),
	}}
	claim.References = []crypto.Hash{submit.AsVersioned().PayloadHash()}
	err = store.LockUTXOs(claim.Inputs, claim.AsVersioned().PayloadHash(), false)
	require.Nil(err)
	err = store.WriteTransaction(claim.AsVersioned())
	require.Nil(err)

	snap = &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       signers[0],
		RoundNumber:  1,
		Timestamp:    uint64(time.Now().UnixNano()),
		Transactions: []crypto.Hash{claim.AsVersioned().PayloadHash()},
		References:   round.References,
	}
	topo = &common.SnapshotWithTopologicalOrder{
		Snapshot:         snap,
		TopologicalOrder: uint64(len(snapshots)) + 2,
	}
	err = store.WriteSnapshot(topo, signers)
	require.Nil(err)

	ver, ss, err = store.ReadWithdrawalClaim(submit.AsVersioned().PayloadHash())
	require.Nil(err)
	require.Equal(topo.PayloadHash().String(), ss)
	require.Equal(claim.AsVersioned().PayloadHash(), ver.PayloadHash())

	_, balance, err = store.ReadAssetWithBalance(common.XINAssetId)
	require.Nil(err)
	require.Equal("365562.00000000", balance.String())

	cs, referencedBy, err := store.ReadLastConsensusSnapshot()
	require.Nil(err)
	require.Nil(referencedBy)
	require.Equal(cs.PayloadHash(), snapshots[len(snapshots)-1].PayloadHash())

	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddUniversalMintInput(0, common.Zero)
	tx.References = []crypto.Hash{cs.SoleTransaction()}
	ver = tx.AsVersioned()
	ncs := &common.Snapshot{
		Version:   common.SnapshotVersionCommonEncoding,
		Timestamp: uint64(time.Now().UnixNano()),
	}
	ncs.AddSoleTransaction(ver.PayloadHash())
	err = store.WriteConsensusSnapshot(ncs, ver)
	require.Nil(err)

	oldCS, oldRB := store.readConsensusSnapshot(cs)
	require.Equal(cs.PayloadHash(), oldCS.PayloadHash())
	require.Equal(oldRB.String(), ver.PayloadHash().String())
	oldnCS, oldnRB := store.readConsensusSnapshot(ncs)
	require.Equal(ncs.PayloadHash(), oldnCS.PayloadHash())
	require.Nil(oldnRB)
}

func (s *BadgerStore) readConsensusSnapshot(snap *common.Snapshot) (*common.Snapshot, *crypto.Hash) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphConsensusSnapshotKey(snap.Timestamp, snap.PayloadHash())
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		panic(err)
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		panic(err)
	}
	if len(val) == 0 {
		return snap, nil
	}
	var h crypto.Hash
	copy(h[:], val)
	return snap, &h
}
