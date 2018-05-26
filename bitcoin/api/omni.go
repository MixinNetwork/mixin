package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const (
	omniUSDT      = "6f6d6e69000000000000001f"
	DustThreshold = int64(546)
)

func LocalSignOmniUSDTTransaction(inputs []*UTXO, output string, omniAmount int64, feePerKb int64, changeAddress string) (*Output, error) {
	tx, inputAmount := wire.NewMsgTx(wire.TxVersion), int64(0)

	for _, input := range inputs {
		hash, err := chainhash.NewHashFromStr(input.TransactionHash)
		if err != nil {
			return nil, err
		}
		txIn := &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  *hash,
				Index: input.Index,
			},
			Sequence: 0xffffffff,
		}
		tx.AddTxIn(txIn)
		inputAmount = inputAmount + input.Amount
	}

	nullData, err := hex.DecodeString(omniUSDT + fmt.Sprintf("%016x", omniAmount))
	if err != nil {
		return nil, err
	}
	nullPkScript, err := txscript.NullDataScript(nullData)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(wire.NewTxOut(0, nullPkScript))

	estimatedRawSizeInKb := int64(len(inputs))*160/1024 + 1
	feeToConsumed := feePerKb * estimatedRawSizeInKb
	changeAmount := inputAmount - feeToConsumed - DustThreshold
	if changeAmount < 0 {
		return nil, fmt.Errorf("insuficcient trasaction fee %d %d %d", inputAmount, feePerKb, estimatedRawSizeInKb)
	}
	if output == changeAddress { // TODO this is to ensure omni shift operation
		changeAmount = changeAmount + DustThreshold
		addressPubKeyHash, err := btcutil.DecodeAddress(output, &chaincfg.MainNetParams)
		if err != nil {
			return nil, err
		}
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return nil, err
		}
		tx.AddTxOut(wire.NewTxOut(changeAmount, pkScript))
	} else {
		if changeAmount > feePerKb {
			addressPubKeyHash, err := btcutil.DecodeAddress(changeAddress, &chaincfg.MainNetParams)
			if err != nil {
				return nil, err
			}
			pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
			if err != nil {
				return nil, err
			}
			tx.AddTxOut(wire.NewTxOut(changeAmount, pkScript))
		} else {
			feeToConsumed = inputAmount - DustThreshold
			changeAmount = 0
		}

		addressPubKeyHash, err := btcutil.DecodeAddress(output, &chaincfg.MainNetParams)
		if err != nil {
			return nil, err
		}
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return nil, err
		}
		tx.AddTxOut(wire.NewTxOut(DustThreshold, pkScript))
	}

	for idx, input := range inputs {
		privateKeyBytes, err := hex.DecodeString(input.PrivateKey)
		if err != nil {
			return nil, err
		}
		privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
		if err != nil {
			return nil, err
		}
		addressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeCompressed(), &chaincfg.MainNetParams)
		if err != nil {
			return nil, err
		}
		addressPubKeyHash := addressPubKey.AddressPubKeyHash()
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return nil, err
		}
		sigScript, err := txscript.SignatureScript(tx, idx, pkScript, txscript.SigHashAll, privateKey, true)
		if err != nil {
			return nil, err
		}
		tx.TxIn[idx].SignatureScript = sigScript
	}

	var rawBuffer bytes.Buffer
	err = tx.BtcEncode(&rawBuffer, wire.ProtocolVersion, wire.BaseEncoding)
	if err != nil {
		return nil, err
	}
	rawBytes := rawBuffer.Bytes()
	if rawSizeInKb := int64(len(rawBytes))/1024 + 1; rawSizeInKb > estimatedRawSizeInKb {
		return nil, fmt.Errorf("Bitcoin raw size estimation error %d %d", rawSizeInKb, estimatedRawSizeInKb)
	}
	if estimatedRawSizeInKb > 30 {
		return nil, fmt.Errorf("Bitcoin transaction size too large %d", estimatedRawSizeInKb)
	}
	transactionHash := tx.TxHash().String()
	outputHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", transactionHash, 0)))
	changeHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", transactionHash, 1)))
	return &Output{
		TransactionHash: transactionHash,
		RawTransaction:  hex.EncodeToString(rawBytes),
		Fee:             feeToConsumed,
		OutputIndex:     0,
		OutputHash:      hex.EncodeToString(outputHash[:]),
		ChangeIndex:     1,
		ChangeHash:      hex.EncodeToString(changeHash[:]),
		ChangeAmount:    changeAmount,
	}, nil
}
