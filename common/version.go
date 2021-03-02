package common

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type VersionedTransaction struct {
	SignedTransaction
	BadGenesis *SignedGenesisHackTransaction `json:"-" msgpack:"-"`
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
	return decompressUnmarshalVersionedOne(val)
}

func UnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	var ver VersionedTransaction
	err := MsgpackUnmarshal(val, &ver)
	if err == nil && ver.Version == TxVersion {
		return &ver, nil
	}
	return unmarshalVersionedOne(val)
}

func (ver *VersionedTransaction) CompressMarshal() []byte {
	switch ver.Version {
	case TxVersion:
		return CompressMsgpackMarshalPanic(ver.SignedTransaction)
	case 0, 1:
		return compressMarshalV1(ver)
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) Marshal() []byte {
	switch ver.Version {
	case TxVersion:
		return MsgpackMarshalPanic(ver.SignedTransaction)
	case 0, 1:
		return marshalV1(ver)
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) PayloadMarshal() []byte {
	switch ver.Version {
	case TxVersion:
		return MsgpackMarshalPanic(ver.SignedTransaction.Transaction)
	case 0, 1:
		return payloadMarshalV1(ver)
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) PayloadHash() crypto.Hash {
	return crypto.NewHash(ver.PayloadMarshal())
}
