package common

import (
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

func TestMsgpack(t *testing.T) {
	require := require.New(t)

	amount := "20"
	assetId := "965e5c6e-434c-3fa9-b780-c50f43cd955c"
	utxoHash := "ee12d68f1a95dd4c9b97ab6e8dc3dba84a5b4e61a1d7b4298a63694b630d3109"
	utxoMask := "2026fe0790c66fd81eab8b20126f5d6146461126652be5248c037af7b4ba640c"
	utxoIndex := 1
	utxoAmount := "8293"

	charge := NewIntegerFromString(utxoAmount).Sub(NewIntegerFromString(amount))
	require.Equal("8273.00000000", charge.String())
	err := msgpackUnmarshal(msgpackMarshalPanic(charge), &charge)
	require.Nil(err)
	require.Equal("8273.00000000", charge.String())

	receiver := &MixinKey{
		ViewKey:  "981ec8403e35b3feb829a7734b8cf56a1229bb344f59fa2766453aa17e931f02",
		SpendKey: "c8327d02a2b79c0f15f8d70118836a79b88d9942cabaaa2b90486a49ec07b001",
	}

	sender := &MixinKey{
		ViewKey:  "77ac6731865c29247588b14dff8e163c81dfaac130cc22882b77a0539db00b0f",
		SpendKey: "87be1eeb3b72909b5447a1699af7538fc0a492222d7b8ab98187299adc4d1b0e",
	}

	tx := NewTransactionV2(crypto.NewHash([]byte(assetId)))
	hash, err := crypto.HashFromString(utxoHash)
	require.Nil(err)
	tx.AddInput(hash, utxoIndex)
	tx.AddRandomScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString(amount))
	tx.AddRandomScriptOutput([]*Address{sender.Address()}, NewThresholdScript(1), charge)
	traceId, err := uuid.FromString("e3aa9cb9-4a28-11e9-81dd-f23c91a6e1fc")
	require.Nil(err)
	tx.Extra = traceId.Bytes()
	msg := msgpackMarshalPanic(tx)
	signed := &SignedTransaction{Transaction: *tx}
	mask := parseKeyFromHex(utxoMask)
	view := sender.Address().PrivateViewKey
	spend := sender.Address().PrivateSpendKey
	priv := crypto.DeriveGhostPrivateKey(&mask, &view, &spend, uint64(utxoIndex))
	sig := priv.Sign(msg)
	sigs := map[uint16]*crypto.Signature{0: &sig}
	signed.SignaturesMap = append(signed.SignaturesMap, sigs)
	raw := msgpackMarshalPanic(signed)

	require.Len(hex.EncodeToString(raw), 772)

	var dec SignedTransaction
	err = msgpackUnmarshal(raw, &dec)
	require.Nil(err)
}

func TestMsgpackV1(t *testing.T) {
	require := require.New(t)

	amount := "20"
	assetId := "965e5c6e-434c-3fa9-b780-c50f43cd955c"
	utxoHash := "ee12d68f1a95dd4c9b97ab6e8dc3dba84a5b4e61a1d7b4298a63694b630d3109"
	utxoMask := "2026fe0790c66fd81eab8b20126f5d6146461126652be5248c037af7b4ba640c"
	utxoIndex := 1
	utxoAmount := "8293"

	charge := NewIntegerFromString(utxoAmount).Sub(NewIntegerFromString(amount))
	require.Equal("8273.00000000", charge.String())
	err := msgpackUnmarshal(msgpackMarshalPanic(charge), &charge)
	require.Nil(err)
	require.Equal("8273.00000000", charge.String())

	receiver := &MixinKey{
		ViewKey:  "981ec8403e35b3feb829a7734b8cf56a1229bb344f59fa2766453aa17e931f02",
		SpendKey: "c8327d02a2b79c0f15f8d70118836a79b88d9942cabaaa2b90486a49ec07b001",
	}

	sender := &MixinKey{
		ViewKey:  "77ac6731865c29247588b14dff8e163c81dfaac130cc22882b77a0539db00b0f",
		SpendKey: "87be1eeb3b72909b5447a1699af7538fc0a492222d7b8ab98187299adc4d1b0e",
	}

	tx := NewTransactionV2(crypto.NewHash([]byte(assetId)))
	tx.Version = 1
	hash, err := crypto.HashFromString(utxoHash)
	require.Nil(err)
	tx.AddInput(hash, utxoIndex)
	tx.AddRandomScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString(amount))
	tx.AddRandomScriptOutput([]*Address{sender.Address()}, NewThresholdScript(1), charge)
	traceId, err := uuid.FromString("e3aa9cb9-4a28-11e9-81dd-f23c91a6e1fc")
	require.Nil(err)
	tx.Extra = traceId.Bytes()
	msg := msgpackMarshalPanic(tx)
	signed := &SignedTransactionV1{Transaction: *tx}
	mask := parseKeyFromHex(utxoMask)
	view := sender.Address().PrivateViewKey
	spend := sender.Address().PrivateSpendKey
	priv := crypto.DeriveGhostPrivateKey(&mask, &view, &spend, uint64(utxoIndex))
	sig := priv.Sign(msg)
	signed.SignaturesSliceV1 = append(signed.SignaturesSliceV1, []*crypto.Signature{&sig})
	raw := msgpackMarshalPanic(signed)

	require.Len(hex.EncodeToString(raw), 930)

	var dec SignedTransactionV1
	err = msgpackUnmarshal(raw, &dec)
	require.Nil(err)
}

type MixinKey struct {
	ViewKey  string
	SpendKey string
}

func (mk *MixinKey) Address() *Address {
	a := Address{
		PrivateViewKey:  parseKeyFromHex(mk.ViewKey),
		PrivateSpendKey: parseKeyFromHex(mk.SpendKey),
	}
	a.PublicViewKey = a.PrivateViewKey.Public()
	a.PublicSpendKey = a.PrivateSpendKey.Public()
	return &a
}

func parseKeyFromHex(src string) crypto.Key {
	var key crypto.Key
	data, _ := hex.DecodeString(src)
	copy(key[:], data)
	return key
}
