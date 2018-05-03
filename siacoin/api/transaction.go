package api

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/encoding"
	"github.com/NebulousLabs/Sia/types"
	"mixin.one/number"
)

type UTXO struct {
	OutputId   string
	Amount     number.Decimal
	PrivateKey string
}

type Output struct {
	TransactionHash string
	RawTransaction  string
	Fee             number.Decimal
	ChangeIndex     int64
	ChangeHash      string
}

func LocalEstimateTransactionFee(inputs []*UTXO, feePerKb number.Decimal) number.Decimal {
	estimatedRawSizeInKb := int64(len(inputs))*750/1024 + 1
	return feePerKb.Mul(number.FromString(fmt.Sprint(estimatedRawSizeInKb)))
}

func LocalSignRawTransaction(inputs []*UTXO, output string, amount, feePerKb number.Decimal, changeAddress string) (*Output, error) {
	tx := types.Transaction{}
	var outputUnlockHash types.UnlockHash
	err := outputUnlockHash.LoadString(output)
	if err != nil {
		return nil, fmt.Errorf("Siacoin invalid unlock hash %s", output)
	}
	if outputUnlockHash.String() != output {
		return nil, fmt.Errorf("Siacoin invalid unlock hash %s", output)
	}

	var inputsMap = make(map[string]*UTXO)
	var inputAmount types.Currency
	for _, input := range inputs {
		seed, err := hex.DecodeString(input.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("Siacoin invalid private key %s", input.OutputId)
		}
		var entropy [crypto.EntropySize]byte
		copy(entropy[:], seed)
		var outputHash crypto.Hash
		err = outputHash.LoadString(input.OutputId)
		if err != nil {
			return nil, fmt.Errorf("Siacoin invalid output id %s", input.OutputId)
		}
		_, pk := crypto.GenerateKeyPairDeterministic(entropy)
		tx.SiacoinInputs = append(tx.SiacoinInputs, types.SiacoinInput{
			ParentID: types.SiacoinOutputID(outputHash),
			UnlockConditions: types.UnlockConditions{
				Timelock:           0,
				PublicKeys:         []types.SiaPublicKey{types.Ed25519PublicKey(pk)},
				SignaturesRequired: 1,
			},
		})
		inputCurrency, err := decimalToCurrency(input.Amount)
		if err != nil {
			return nil, fmt.Errorf("Siacoin invalid input amount %s", input.Amount.Persist())
		}
		inputAmount = inputAmount.Add(inputCurrency)
		inputsMap[input.OutputId] = input
	}

	outputAmount, err := decimalToCurrency(amount)
	if err != nil {
		return nil, fmt.Errorf("Siacoin invalid amount %s", amount.Persist())
	}
	tx.SiacoinOutputs = append(tx.SiacoinOutputs, types.SiacoinOutput{
		Value:      outputAmount,
		UnlockHash: outputUnlockHash,
	})
	estimatedRawSizeInKb := len(inputs)*750/1024 + 1
	feeAmount := feePerKb.Mul(number.FromString(fmt.Sprint(estimatedRawSizeInKb)))
	feeToConsumed, err := decimalToCurrency(feeAmount)
	if err != nil {
		return nil, fmt.Errorf("Siacoin invalid fee %s", feeAmount.String())
	}
	changeAmount := inputAmount.Sub(outputAmount).Sub(feeToConsumed)
	if changeAmount.Cmp64(0) < 0 {
		return nil, fmt.Errorf("Siacoin insuficcient trasaction fee %s %s %f", inputAmount.String(), feePerKb.String(), estimatedRawSizeInKb)
	}
	if number.FromString(changeAmount.String()).Cmp(feePerKb.Mul(number.New(1, -24))) > 0 {
		var changeUnlockHash types.UnlockHash
		err := changeUnlockHash.LoadString(changeAddress)
		if err != nil {
			return nil, fmt.Errorf("Siacoin invalid change unlock hash %s", changeAddress)
		}
		if changeUnlockHash.String() != changeAddress {
			return nil, fmt.Errorf("Siacoin invalid change unlock hash %s", changeAddress)
		}
		tx.SiacoinOutputs = append(tx.SiacoinOutputs, types.SiacoinOutput{
			Value:      changeAmount,
			UnlockHash: changeUnlockHash,
		})
	} else {
		feeToConsumed = inputAmount.Sub(outputAmount)
	}
	tx.MinerFees = append(tx.MinerFees, feeToConsumed)
	for _, si := range tx.SiacoinInputs {
		input := inputsMap[si.ParentID.String()]
		seed, err := hex.DecodeString(input.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("Siacoin invalid private key %s", input.OutputId)
		}
		var entropy [crypto.EntropySize]byte
		copy(entropy[:], seed)
		var outputHash crypto.Hash
		err = outputHash.LoadString(input.OutputId)
		if err != nil {
			return nil, fmt.Errorf("Siacoin invalid output id %s", input.OutputId)
		}
		sk, _ := crypto.GenerateKeyPairDeterministic(entropy)
		tx.TransactionSignatures = append(tx.TransactionSignatures, types.TransactionSignature{
			ParentID:       outputHash,
			CoveredFields:  types.CoveredFields{WholeTransaction: true},
			PublicKeyIndex: 0,
		})
		sigIndex := len(tx.TransactionSignatures) - 1
		sigHash := tx.SigHash(sigIndex)
		encodedSig := crypto.SignHash(sigHash, sk)
		tx.TransactionSignatures[sigIndex].Signature = encodedSig[:]
	}
	result := &Output{
		TransactionHash: tx.ID().String(),
		RawTransaction:  hex.EncodeToString(encoding.Marshal(tx)),
		Fee:             number.FromString(feeToConsumed.String()).Mul(number.New(1, 24)),
	}
	if len(tx.SiacoinOutputs) > 1 {
		result.ChangeIndex = 1
		result.ChangeHash = tx.SiacoinOutputID(uint64(result.ChangeIndex)).String()
	}
	if estimatedRawSizeInKb > 30 {
		return nil, fmt.Errorf("Siacoin transaction size too large %d", estimatedRawSizeInKb)
	}
	return result, nil
}

func decimalToCurrency(amount number.Decimal) (types.Currency, error) {
	bigAmount, success := new(big.Int).SetString(amount.Mul(number.New(1, -24)).RoundFloor(0).String(), 10)
	if !success {
		return types.Currency{}, fmt.Errorf("Siacoin invalid amount parse %s", amount.Persist())
	}
	return types.NewCurrency(bigAmount), nil
}
