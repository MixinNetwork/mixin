package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/util/base58"
	"github.com/stretchr/testify/require"
)

func TestDecoderHelperErrors(t *testing.T) {
	require := require.New(t)

	_, err := NewMinimumDecoder([]byte{1, 2, 3})
	require.ErrorContains(err, "invalid encoding version")

	_, err = NewMinimumDecoder([]byte{0, 0, 0, 0})
	require.ErrorContains(err, "invalid encoding version")

	ok, err := NewDecoder([]byte{0x12, 0x34}).ReadMagic()
	require.False(ok)
	require.ErrorContains(err, "malformed")

	enc := NewEncoder()
	enc.WriteInt(1)
	_, err = NewDecoder(enc.Bytes()).ReadRoundReferences()
	require.ErrorContains(err, "invalid references count 1")

	enc = NewEncoder()
	enc.Write(make([]byte, len(crypto.Signature{})))
	_ = enc.WriteByte(0xff)
	_, err = NewDecoder(enc.Bytes()).ReadAggregatedSignature()
	require.ErrorContains(err, "invalid mask type 255")

	enc = NewEncoder()
	enc.WriteInt(2)
	enc.WriteUint16(0)
	enc.Write(make([]byte, len(crypto.Signature{})))
	enc.WriteUint16(0)
	enc.Write(make([]byte, len(crypto.Signature{})))
	_, err = NewDecoder(enc.Bytes()).ReadSignatures()
	require.ErrorContains(err, "signatures count 2")

	enc = NewEncoder()
	enc.Write(magic)
	enc.Write([]byte{0x00, TxVersionHashSignature})
	enc.Write(XINAssetId[:])
	enc.WriteInt(0)
	enc.WriteInt(0)
	enc.WriteInt(0)
	enc.WriteUint32(0)
	enc.WriteInt(MaximumEncodingInt)
	enc.WriteInt(123)
	_, err = NewDecoder(enc.Bytes()).DecodeTransaction()
	require.ErrorContains(err, "invalid prefix 123")

	enc = NewEncoder()
	_ = enc.WriteByte(0x7f)
	b, err := NewDecoder(enc.Bytes()).ReadByte()
	require.Nil(err)
	require.Equal(byte(0x7f), b)
}

func TestEncoderPanics(t *testing.T) {
	require := require.New(t)

	require.Panics(func() {
		NewEncoder().WriteInt(MaximumEncodingInt + 1)
	})

	require.Panics(func() {
		NewEncoder().EncodeCosiSignature(&crypto.CosiSignature{})
	})

	require.Panics(func() {
		NewEncoder().EncodeAggregatedSignature(&AggregatedSignature{Signers: []int{2, 1}})
	})

	s := &Snapshot{
		Version:      SnapshotVersionCommonEncoding,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("snapshot tx"))},
		Signature:    &crypto.CosiSignature{Mask: 1},
	}
	require.Panics(func() {
		NewEncoder().EncodeSnapshotPayload(s)
	})
}

func TestInputAndUTXORoundTrip(t *testing.T) {
	require := require.New(t)

	input := &Input{
		Hash:    crypto.Blake3Hash([]byte("input-hash")),
		Index:   5,
		Genesis: []byte("genesis"),
		Deposit: &DepositData{
			Chain:       BitcoinAssetId,
			AssetKey:    "btc",
			Transaction: "deposit-hash",
			Index:       9,
			Amount:      NewInteger(42),
		},
		Mint: &MintData{
			Group:  mintGroupUniversal,
			Batch:  7,
			Amount: NewInteger(11),
		},
	}

	enc := NewEncoder()
	enc.EncodeInput(input)
	decodedInput, err := NewDecoder(enc.Bytes()).ReadInput()
	require.Nil(err)
	require.Equal(input, decodedInput)

	mask := crypto.NewKeyFromSeed([]byte("0123456789012345678901234567890101234567890123456789012345678901")).Public()
	key1 := crypto.NewKeyFromSeed([]byte("1123456789012345678901234567890111234567890123456789012345678901")).Public()
	key2 := crypto.NewKeyFromSeed([]byte("2123456789012345678901234567890121234567890123456789012345678901")).Public()
	utxo := &UTXOWithLock{
		UTXO: UTXO{
			Input: *input,
			Output: Output{
				Type:   OutputTypeScript,
				Amount: NewInteger(99),
				Keys:   []*crypto.Key{&key1, &key2},
				Mask:   mask,
				Script: NewThresholdScript(2),
			},
			Asset: XINAssetId,
		},
		LockHash: crypto.Blake3Hash([]byte("lock-hash")),
	}

	decodedUTXO, err := UnmarshalUTXO(utxo.Marshal())
	require.Nil(err)
	require.Equal(utxo, decodedUTXO)
}

func TestRoundHelpersAndHash(t *testing.T) {
	require := require.New(t)

	link := &RoundLink{
		Self:     crypto.Blake3Hash([]byte("self")),
		External: crypto.Blake3Hash([]byte("external")),
	}
	copyLink := link.Copy()
	require.NotSame(link, copyLink)
	require.True(link.Equal(copyLink))
	copyLink.Self = crypto.Blake3Hash([]byte("other"))
	require.False(link.Equal(copyLink))

	nodeID := crypto.Blake3Hash([]byte("round-node"))
	s1 := &Snapshot{Version: SnapshotVersionCommonEncoding, Timestamp: 30, Hash: crypto.Blake3Hash([]byte("snapshot-1"))}
	s2 := &Snapshot{Version: SnapshotVersionCommonEncoding, Timestamp: 10, Hash: crypto.Blake3Hash([]byte("snapshot-2"))}
	s3 := &Snapshot{Version: SnapshotVersionCommonEncoding, Timestamp: 30, Hash: crypto.Blake3Hash([]byte("snapshot-0"))}
	start, end, hash := ComputeRoundHash(nodeID, 7, []*Snapshot{s1, s2, s3})
	require.Equal(uint64(10), start)
	require.Equal(uint64(30), end)

	start2, end2, hash2 := ComputeRoundHash(nodeID, 7, []*Snapshot{s3, s1, s2})
	require.Equal(start, start2)
	require.Equal(end, end2)
	require.Equal(hash, hash2)

	require.Panics(func() {
		ComputeRoundHash(nodeID, 8, []*Snapshot{
			{Version: SnapshotVersionCommonEncoding, Timestamp: 1, Hash: crypto.Blake3Hash([]byte("early"))},
			{Version: SnapshotVersionCommonEncoding, Timestamp: 1 + config.SnapshotRoundGap, Hash: crypto.Blake3Hash([]byte("late"))},
		})
	})
}

func TestSnapshotHelpers(t *testing.T) {
	require := require.New(t)

	tx := crypto.Blake3Hash([]byte("sole-transaction"))
	s := &Snapshot{Version: SnapshotVersionCommonEncoding}
	s.AddSoleTransaction(tx)
	require.Equal(tx, s.SoleTransaction())

	decoded, err := UnmarshalVersionedSnapshot(s.VersionedMarshal())
	require.Nil(err)
	require.Equal(tx, decoded.SoleTransaction())
	require.Equal(uint64(0), decoded.TopologicalOrder)

	require.Panics(func() {
		s.AddSoleTransaction(crypto.Blake3Hash([]byte("extra")))
	})

	require.Panics(func() {
		(&Snapshot{Version: 1}).AddSoleTransaction(tx)
	})

	require.Panics(func() {
		(&Snapshot{Version: 1}).SoleTransaction()
	})

	require.Panics(func() {
		(&Snapshot{
			Version:      SnapshotVersionCommonEncoding,
			Transactions: []crypto.Hash{tx, crypto.Blake3Hash([]byte("second"))},
		}).SoleTransaction()
	})
}

func TestRationalAssetAndDepositHelpers(t *testing.T) {
	require := require.New(t)

	ratio := NewInteger(2).Ration(NewInteger(5))
	require.Equal("0.40000000", ratio.String())

	require.Equal("2500.00000000", GetAssetCapacity(BitcoinAssetId).String())
	require.Equal(
		"115792089237316195423570985008687907853269984665640564039457.58400791",
		GetAssetCapacity(crypto.Blake3Hash([]byte("unknown-asset"))).String(),
	)

	require.Nil((&Asset{Chain: BitcoinAssetId, AssetKey: "btc"}).Verify())
	require.ErrorContains((&Asset{AssetKey: "btc"}).Verify(), "invalid asset chain")
	require.ErrorContains((&Asset{Chain: BitcoinAssetId, AssetKey: " btc "}).Verify(), "invalid asset key")

	deposit := &DepositData{
		Chain:       BitcoinAssetId,
		AssetKey:    "btc",
		Transaction: "deposit-hash",
		Index:       1,
	}
	key := deposit.UniqueKey()
	require.True(key.HasValue())
	require.Equal(key, deposit.UniqueKey())

	other := *deposit
	other.Index = 2
	require.NotEqual(key, other.UniqueKey())
}

func TestMintAndLockInputHelpers(t *testing.T) {
	require := require.New(t)

	mint := &MintData{
		Group:  mintGroupUniversal,
		Batch:  9,
		Amount: NewInteger(3),
	}
	dist := mint.Distribute(crypto.Blake3Hash([]byte("mint-transaction")))
	require.Equal(mint.Group, dist.Group)
	require.Equal(mint.Batch, dist.Batch)
	require.Equal(mint.Amount, dist.Amount)

	mintTx := NewTransactionV5(XINAssetId)
	mintTx.AddUniversalMintInput(mint.Batch, mint.Amount)
	mintVer := mintTx.AsVersioned()
	require.EqualValues(TransactionTypeMint, mintVer.TransactionType())

	locker := &recordingLocker{}
	err := mintVer.LockInputs(locker, true)
	require.Nil(err)
	require.Equal("mint", locker.kind)
	require.Equal(mint.Batch, locker.mint.Batch)
	require.True(locker.tx.HasValue())
	require.True(locker.fork)

	deposit := &DepositData{
		Chain:       BitcoinAssetId,
		AssetKey:    "btc",
		Transaction: "deposit-lock",
		Index:       7,
		Amount:      NewInteger(5),
	}
	depositTx := NewTransactionV5(XINAssetId)
	depositTx.AddDepositInput(deposit)
	depositVer := depositTx.AsVersioned()
	require.EqualValues(TransactionTypeDeposit, depositVer.TransactionType())
	require.Equal(deposit, depositTx.DepositData())

	locker = &recordingLocker{}
	err = depositVer.LockInputs(locker, false)
	require.Nil(err)
	require.Equal("deposit", locker.kind)
	require.Equal(deposit, locker.deposit)

	scriptTx := NewTransactionV5(XINAssetId)
	scriptTx.AddInput(crypto.Blake3Hash([]byte("input")), 0)
	scriptTx.AddScriptOutput(nil, NewThresholdScript(1), NewInteger(1), nil)
	scriptVer := scriptTx.AsVersioned()
	require.EqualValues(TransactionTypeScript, scriptVer.TransactionType())

	locker = &recordingLocker{}
	err = scriptVer.LockInputs(locker, false)
	require.Nil(err)
	require.Equal("utxo", locker.kind)
	require.Len(locker.inputs, 1)

	withdrawalTx := NewTransactionV5(XINAssetId)
	withdrawalTx.Outputs = []*Output{{Type: OutputTypeWithdrawalSubmit, Amount: NewInteger(1)}}
	require.EqualValues(TransactionTypeWithdrawalSubmit, withdrawalTx.AsVersioned().TransactionType())

	unknownTx := NewTransactionV5(XINAssetId)
	unknownTx.Inputs = []*Input{{Genesis: []byte("genesis")}}
	require.EqualValues(TransactionTypeUnknown, unknownTx.AsVersioned().TransactionType())
}

func TestAddressAndMintParsingEdges(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i + 3)
	}
	addr := NewAddressFromSeed(seed).String()

	_, err := NewAddressFromString("ABC")
	require.ErrorContains(err, "invalid address network")

	_, err = NewAddressFromString(MainAddressPrefix + "1")
	require.ErrorContains(err, "invalid address format")

	badSpend := mutateAddressForTest(addr, func(data []byte) {
		copy(data[:32], make([]byte, 32))
	})
	_, err = NewAddressFromString(badSpend)
	require.ErrorContains(err, "invalid address public spend key")

	badView := mutateAddressForTest(addr, func(data []byte) {
		copy(data[32:64], make([]byte, 32))
	})
	_, err = NewAddressFromString(badView)
	require.ErrorContains(err, "invalid address public view key")

	require.Panics(func() {
		(&MintDistribution{
			MintData: MintData{
				Group:  "invalid",
				Batch:  1,
				Amount: NewInteger(1),
			},
		}).Marshal()
	})

	_, err = UnmarshalMintDistribution([]byte{1, 2, 3})
	require.ErrorContains(err, "invalid mint distribution size")

	enc := NewMinimumEncoder()
	enc.WriteUint16(9)
	enc.WriteUint64(1)
	enc.WriteInteger(NewInteger(1))
	hash := crypto.Blake3Hash([]byte("mint-distribution"))
	enc.Write(hash[:])
	_, err = UnmarshalMintDistribution(enc.Bytes())
	require.ErrorContains(err, "invalid mint distribution group")
}

func TestDecoderAndSnapshotEdgeCoverage(t *testing.T) {
	require := require.New(t)

	validTx := NewTransactionV5(XINAssetId).AsVersioned().Marshal()
	_, err := NewDecoder(append(validTx, 1)).DecodeTransaction()
	require.ErrorContains(err, "unexpected ending")

	enc := NewEncoder()
	enc.Write([]byte{1, byte(OutputTypeScript)})
	_, err = NewDecoder(enc.Bytes()).ReadOutput()
	require.ErrorContains(err, "invalid output type")

	enc = NewEncoder()
	enc.WriteUint64(0)
	cs, err := NewDecoder(enc.Bytes()).ReadCosiSignature()
	require.Nil(err)
	require.Nil(cs)

	enc = NewEncoder()
	enc.WriteInt(0)
	refs, err := NewDecoder(enc.Bytes()).ReadRoundReferences()
	require.Nil(err)
	require.Nil(refs)

	enc = NewEncoder()
	enc.Write(magic)
	enc.Write([]byte{0x00, SnapshotVersionCommonEncoding})
	node := crypto.Blake3Hash([]byte("node"))
	enc.Write(node[:])
	enc.WriteUint64(7)
	enc.EncodeRoundReferences(&RoundLink{
		Self:     crypto.Blake3Hash([]byte("self")),
		External: crypto.Blake3Hash([]byte("external")),
	})
	enc.WriteInt(0)
	_, err = NewDecoder(enc.Bytes()).DecodeSnapshotWithTopo()
	require.ErrorContains(err, "invalid transactions count 0")

	snapshot := &Snapshot{
		Version:      SnapshotVersionCommonEncoding,
		NodeId:       crypto.Blake3Hash([]byte("snapshot-node")),
		RoundNumber:  9,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("snapshot-tx"))},
		Timestamp:    11,
	}
	payloadOnly := NewEncoder().EncodeSnapshotPayload(snapshot)
	topo, err := NewDecoder(payloadOnly).DecodeSnapshotWithTopo()
	require.Nil(err)
	require.Equal(uint64(0), topo.TopologicalOrder)

	versioned := (&SnapshotWithTopologicalOrder{Snapshot: snapshot, TopologicalOrder: 3}).VersionedMarshal()
	_, err = NewDecoder(append(versioned, 1)).DecodeSnapshotWithTopo()
	require.ErrorContains(err, "unexpected ending")

	require.Panics(func() {
		(&Snapshot{Version: 1}).PayloadHash()
	})
}

func TestVersionedTransactionAndUTXOEdgeCoverage(t *testing.T) {
	require := require.New(t)

	ver := NewTransactionV5(XINAssetId).AsVersioned()
	ver.pmbytes = []byte{7, 8, 9}
	require.Equal([]byte{7, 8, 9}, ver.PayloadMarshal())

	_, err := UnmarshalVersionedTransaction(make([]byte, config.TransactionMaximumSize+1))
	require.ErrorContains(err, "transaction too large")

	tx := NewTransactionV5(XINAssetId)
	tx.Outputs = []*Output{
		{Type: OutputTypeScript, Amount: NewInteger(1)},
		{Type: OutputTypeWithdrawalSubmit, Amount: NewInteger(2)},
		{Type: OutputTypeCustodianSlashNodes, Amount: NewInteger(3)},
	}
	utxos := tx.AsVersioned().UnspentOutputs()
	require.Len(utxos, 1)
	require.Equal(uint(0), utxos[0].Index)

	require.Panics(func() {
		panicTx := NewTransactionV5(XINAssetId)
		panicTx.Outputs = []*Output{{Type: 0xff, Amount: NewInteger(1)}}
		panicTx.AsVersioned().UnspentOutputs()
	})

	_, err = UnmarshalUTXO(make([]byte, 16))
	require.ErrorContains(err, "invalid encoding version")

	truncated := append(NewMinimumEncoder().Bytes(), make([]byte, 12)...)
	_, err = UnmarshalUTXO(truncated)
	require.ErrorContains(err, "data short")
}

func mutateAddressForTest(addr string, mutate func([]byte)) string {
	data := base58.Decode(addr[len(MainAddressPrefix):])
	cloned := append([]byte{}, data...)
	mutate(cloned[:64])
	checksum := crypto.Sha256Hash(append([]byte(MainAddressPrefix), cloned[:64]...))
	copy(cloned[64:], checksum[:4])
	return MainAddressPrefix + base58.Encode(cloned)
}

type recordingLocker struct {
	kind    string
	inputs  []*Input
	deposit *DepositData
	mint    *MintData
	tx      crypto.Hash
	fork    bool
}

func (l *recordingLocker) LockUTXOs(inputs []*Input, tx crypto.Hash, fork bool) error {
	l.kind = "utxo"
	l.inputs = inputs
	l.tx = tx
	l.fork = fork
	return nil
}

func (l *recordingLocker) LockDepositInput(deposit *DepositData, tx crypto.Hash, fork bool) error {
	l.kind = "deposit"
	l.deposit = deposit
	l.tx = tx
	l.fork = fork
	return nil
}

func (l *recordingLocker) LockMintInput(mint *MintData, tx crypto.Hash, fork bool) error {
	l.kind = "mint"
	l.mint = mint
	l.tx = tx
	l.fork = fork
	return nil
}
