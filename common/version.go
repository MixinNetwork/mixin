package common

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

type VersionedTransaction struct {
	SignedTransaction
	BadGenesis *SignedGenesisHackTransaction `msgpack:"-"`
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
	ver, err := decompressUnmarshalVersionedTransaction(val)
	if err != nil {
		return nil, err
	}
	if config.Debug {
		ret1 := ver.compressMarshal()
		ret2 := ver.marshal()
		if !bytes.Equal(val, ret1) && !bytes.Equal(val, ret2) {
			return nil, fmt.Errorf("malformed %d %d %d", len(val), len(ret1), len(ret2))
		}
	}
	return ver, nil
}

func UnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	ver, err := unmarshalVersionedTransaction(val)
	if err != nil {
		return nil, err
	}
	if config.Debug {
		ret := ver.marshal()
		if !bytes.Equal(val, ret) {
			return nil, fmt.Errorf("malformed %d %d", len(ret), len(val))
		}
	}
	return ver, nil
}

func (ver *VersionedTransaction) CompressMarshal() []byte {
	val := ver.compressMarshal()
	if config.Debug {
		ret, err := decompressUnmarshalVersionedTransaction(val)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(ret.compressMarshal(), val) {
			panic(fmt.Errorf("malformed %d", len(val)))
		}
	}
	return val
}

func (ver *VersionedTransaction) Marshal() []byte {
	val := ver.marshal()
	if config.Debug {
		ret, err := unmarshalVersionedTransaction(val)
		if err != nil {
			panic(err)
		}
		if retv := ret.marshal(); !bytes.Equal(retv, val) {
			panic(fmt.Errorf("malformed %s %s", hex.EncodeToString(val), hex.EncodeToString(retv)))
		}
	}
	return val
}

func (ver *VersionedTransaction) PayloadMarshal() []byte {
	val := ver.payloadMarshal()
	if config.Debug {
		ret, err := unmarshalVersionedTransaction(val)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(ret.payloadMarshal(), val) {
			panic(fmt.Errorf("malformed %d", len(val)))
		}
	}
	return val
}

func (ver *VersionedTransaction) PayloadHash() crypto.Hash {
	return crypto.NewHash(ver.PayloadMarshal())
}

func decompressUnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	var ver VersionedTransaction
	err := DecompressMsgpackUnmarshal(val, &ver)
	if err == nil && ver.Version == TxVersion {
		return &ver, nil
	}
	return decompressUnmarshalVersionedOne(val)
}

func unmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	var ver VersionedTransaction
	err := MsgpackUnmarshal(val, &ver)
	if err == nil && ver.Version == TxVersion {
		return &ver, nil
	}
	return unmarshalVersionedOne(val)
}

func (ver *VersionedTransaction) compressMarshal() []byte {
	switch ver.Version {
	case TxVersion:
		return CompressMsgpackMarshalPanic(ver.SignedTransaction)
	case 0, 1:
		return compressMarshalV1(ver)
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) marshal() []byte {
	switch ver.Version {
	case TxVersion:
		return MsgpackMarshalPanic(ver.SignedTransaction)
	case 0, 1:
		return marshalV1(ver)
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) payloadMarshal() []byte {
	switch ver.Version {
	case TxVersion:
		return MsgpackMarshalPanic(ver.SignedTransaction.Transaction)
	case 0, 1:
		return payloadMarshalV1(ver)
	default:
		panic(ver.Version)
	}
}
