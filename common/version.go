package common

import (
	"encoding/hex"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

type VersionedTransaction struct {
	SignedTransaction
	V1 *VersionedOne
}

func (tx *SignedTransaction) AsLatestVersion() *VersionedTransaction {
	if tx.Version != TxVersion {
		panic(tx.Version)
	}
	return &VersionedTransaction{
		SignedTransaction: *tx,
	}
}

func (tx *Transaction) AsLatestVersion() *VersionedTransaction {
	if tx.Version != TxVersion {
		panic(tx.Version)
	}
	return &VersionedTransaction{
		SignedTransaction: SignedTransaction{Transaction: *tx},
	}
}

func DecompressUnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	var ver VersionedTransaction
	err := DecompressMsgpackUnmarshal(val, &ver)
	if err == nil && ver.Version == TxVersion {
		return &ver, nil
	}
	v1, err := decompressUnmarshalVersionedOne(val)
	if err != nil {
		return nil, err
	}
	ver.Version = v1.Version
	ver.V1 = v1
	return &ver, nil
}

func UnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	var ver VersionedTransaction
	err := MsgpackUnmarshal(val, &ver)
	if err == nil && ver.Version == TxVersion {
		return &ver, nil
	}
	v1, err := unmarshalVersionedOne(val)
	if err != nil {
		return nil, err
	}
	ver.Version = v1.Version
	ver.V1 = v1
	return &ver, nil
}

func (ver *VersionedTransaction) CompressMarshal() []byte {
	switch ver.Version {
	case TxVersion:
		return CompressMsgpackMarshalPanic(ver.SignedTransaction)
	case 0, 1:
		return ver.V1.compressMarshal()
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) Marshal() []byte {
	switch ver.Version {
	case TxVersion:
		return MsgpackMarshalPanic(ver.SignedTransaction)
	case 0, 1:
		return ver.V1.marshal()
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) PayloadMarshal() []byte {
	switch ver.Version {
	case TxVersion:
		return MsgpackMarshalPanic(ver.SignedTransaction.Transaction)
	case 0, 1:
		return ver.V1.payloadMarshal()
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) PayloadHash() crypto.Hash {
	return crypto.NewHash(ver.PayloadMarshal())
}

type VersionedOne struct {
	SignedTransaction
	BadGenesis *SignedGenesisHackTransaction
}

func decompressUnmarshalVersionedOne(val []byte) (*VersionedOne, error) {
	var tx SignedTransaction
	err := DecompressMsgpackUnmarshal(val, &tx)
	if err != nil {
		return nil, err
	}

	ver := &VersionedOne{
		SignedTransaction: tx,
	}

	if tx.Version == 1 && len(tx.Inputs) == 1 && hex.EncodeToString(tx.Inputs[0].Genesis) == config.MainnetId {
		var ght SignedGenesisHackTransaction
		err := DecompressMsgpackUnmarshal(val, &ght)
		if err != nil {
			return nil, err
		}
		ver.Version = 0
		ver.BadGenesis = &ght
	}
	return ver, nil
}

func unmarshalVersionedOne(val []byte) (*VersionedOne, error) {
	var tx SignedTransaction
	err := MsgpackUnmarshal(val, &tx)
	if err != nil {
		return nil, err
	}

	ver := &VersionedOne{
		SignedTransaction: tx,
	}

	if tx.Version == 1 && len(tx.Inputs) == 1 && hex.EncodeToString(tx.Inputs[0].Genesis) == config.MainnetId {
		var ght SignedGenesisHackTransaction
		err := MsgpackUnmarshal(val, &ght)
		if err != nil {
			return nil, err
		}
		ver.Version = 0
		ver.BadGenesis = &ght
	}
	return ver, nil
}

func (ver *VersionedOne) compressMarshal() []byte {
	var msg []byte
	switch ver.Version {
	case 0:
		msg = CompressMsgpackMarshalPanic(ver.BadGenesis)
	case TxVersion:
		msg = CompressMsgpackMarshalPanic(ver.SignedTransaction)
	}
	return msg
}

func (ver *VersionedOne) marshal() []byte {
	var msg []byte
	switch ver.Version {
	case 0:
		msg = MsgpackMarshalPanic(ver.BadGenesis)
	case TxVersion:
		msg = MsgpackMarshalPanic(ver.SignedTransaction)
	}
	return msg
}

func (ver *VersionedOne) payloadMarshal() []byte {
	var msg []byte
	switch ver.Version {
	case 0:
		msg = MsgpackMarshalPanic(ver.BadGenesis.GenesisHackTransaction)
	case TxVersion:
		msg = MsgpackMarshalPanic(ver.SignedTransaction.Transaction)
	}
	return msg
}

type GenesisHackInput struct {
	Hash    crypto.Hash
	Index   int
	Genesis []byte
	Deposit *DepositData
	Rebate  []byte
	Mint    []byte
}

type GenesisHackTransaction struct {
	Version uint8
	Asset   crypto.Hash
	Inputs  []*GenesisHackInput
	Outputs []*Output
	Extra   []byte
}

type SignedGenesisHackTransaction struct {
	GenesisHackTransaction
	Signatures [][]crypto.Signature
}
