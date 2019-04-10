package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (ver *VersionedTransaction) Validate(store DataStore) error {
	tx := &ver.SignedTransaction
	msg := ver.PayloadMarshal()
	txType := tx.TransactionType()

	if ver.Version != TxVersion || tx.Version != TxVersion {
		return fmt.Errorf("invalid tx version %d %d", ver.Version, tx.Version)
	}
	if txType == TransactionTypeUnknown {
		return fmt.Errorf("invalid tx type %d", txType)
	}
	if len(tx.Inputs) < 1 || len(tx.Outputs) < 1 {
		return fmt.Errorf("invalid tx inputs or outputs %d %d", len(tx.Inputs), len(tx.Outputs))
	}
	if len(tx.Inputs) != len(tx.Signatures) {
		return fmt.Errorf("invalid tx signature number %d %d", len(tx.Inputs), len(tx.Signatures))
	}
	if len(tx.Extra) > ExtraSizeLimit {
		return fmt.Errorf("invalid extra size %d", len(tx.Extra))
	}
	if len(msg) > config.TransactionMaximumSize {
		return fmt.Errorf("invalid transaction size %d", len(msg))
	}

	inputsFilter, inputAmount, err := validateInputs(store, tx, msg)
	if err != nil {
		return err
	}
	outputAmount, err := validateOutputs(store, tx)
	if err != nil {
		return err
	}

	if inputAmount.Sign() <= 0 || inputAmount.Cmp(outputAmount) != 0 {
		return fmt.Errorf("invalid input output amount %s %s", inputAmount.String(), outputAmount.String())
	}

	switch txType {
	case TransactionTypeScript:
		return validateScriptTransaction(inputsFilter)
	case TransactionTypeMint:
		return ver.validateMint(store)
	case TransactionTypeDeposit:
		return tx.validateDeposit(store, msg, ver.PayloadHash())
	case TransactionTypeWithdrawalSubmit:
		return tx.validateWithdrawalSubmit(inputsFilter)
	case TransactionTypeWithdrawalFuel:
		return tx.validateWithdrawalFuel(store, inputsFilter)
	case TransactionTypeWithdrawalClaim:
		return tx.validateWithdrawalClaim(store, inputsFilter, msg)
	case TransactionTypeNodePledge:
		return tx.validateNodePledge(store, inputsFilter)
	case TransactionTypeNodeAccept:
		return tx.validateNodeAccept(store)
	case TransactionTypeNodeDepart:
		return fmt.Errorf("invalid transaction type %d", txType)
	case TransactionTypeNodeRemove:
		return fmt.Errorf("invalid transaction type %d", txType)
	case TransactionTypeDomainAccept:
		return fmt.Errorf("invalid transaction type %d", txType)
	case TransactionTypeDomainRemove:
		return fmt.Errorf("invalid transaction type %d", txType)
	}
	return fmt.Errorf("invalid transaction type %d", txType)
}

func validateScriptTransaction(inputs map[string]*UTXO) error {
	for _, in := range inputs {
		if in.Type != OutputTypeScript {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}
	return nil
}

func validateInputs(store DataStore, tx *SignedTransaction, msg []byte) (map[string]*UTXO, Integer, error) {
	inputAmount := NewInteger(0)
	inputsFilter := make(map[string]*UTXO)

	for i, in := range tx.Inputs {
		if in.Mint != nil {
			return inputsFilter, in.Mint.Amount, nil
		}

		if in.Deposit != nil {
			return inputsFilter, in.Deposit.Amount, nil
		}

		fk := fmt.Sprintf("%s:%d", in.Hash.String(), in.Index)
		if inputsFilter[fk] != nil {
			return inputsFilter, inputAmount, fmt.Errorf("invalid input %s", fk)
		}

		utxo, err := store.ReadUTXO(in.Hash, in.Index)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		if utxo == nil {
			return inputsFilter, inputAmount, fmt.Errorf("input not found %s:%d", in.Hash.String(), in.Index)
		}
		if utxo.Asset != tx.Asset {
			return inputsFilter, inputAmount, fmt.Errorf("invalid input asset %s %s", utxo.Asset.String(), tx.Asset.String())
		}

		err = validateUTXO(utxo, tx.Signatures[i], msg)
		if err != nil {
			return inputsFilter, inputAmount, err
		}
		inputsFilter[fk] = utxo
		inputAmount = inputAmount.Add(utxo.Amount)
	}

	return inputsFilter, inputAmount, nil
}

func validateOutputs(store DataStore, tx *SignedTransaction) (Integer, error) {
	outputAmount := NewInteger(0)
	outputsFilter := make(map[crypto.Key]bool)
	for _, o := range tx.Outputs {
		if o.Withdrawal != nil {
			continue
		}

		if o.Amount.Sign() <= 0 {
			return outputAmount, fmt.Errorf("invalid output amount %s", o.Amount.String())
		}

		for _, k := range o.Keys {
			if outputsFilter[k] {
				return outputAmount, fmt.Errorf("invalid output key %s", k.String())
			}
			outputsFilter[k] = true
			exist, err := store.CheckGhost(k)
			if err != nil {
				return outputAmount, err
			} else if exist {
				return outputAmount, fmt.Errorf("invalid output key %s", k.String())
			}
		}

		if o.Type == OutputTypeScript {
			err := o.Script.VerifyFormat()
			if err != nil {
				return outputAmount, err
			}
		}
		outputAmount = outputAmount.Add(o.Amount)
	}
	return outputAmount, nil
}

func validateUTXO(utxo *UTXO, sigs []crypto.Signature, msg []byte) error {
	switch utxo.Type {
	case OutputTypeScript:
	case OutputTypeNodePledge:
	case OutputTypeNodeAccept:
	default:
		return fmt.Errorf("invalid input type %d", utxo.Type)
	}

	var offset, valid int
	for _, sig := range sigs {
		for i, k := range utxo.Keys {
			if i < offset {
				continue
			}
			if k.Verify(msg, sig) {
				valid = valid + 1
				offset = i + 1
			}
		}
	}

	return utxo.Script.Validate(valid)
}
