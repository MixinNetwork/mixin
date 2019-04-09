package common

import (
	"crypto/rand"
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	TxVersion      = 0x01
	ExtraSizeLimit = 256

	OutputTypeScript           = 0x00
	OutputTypeWithdrawalSubmit = 0xa1
	OutputTypeWithdrawalFuel   = 0xa2
	OutputTypeNodePledge       = 0xa3
	OutputTypeNodeAccept       = 0xa4
	OutputTypeNodeDepart       = 0xa5
	OutputTypeNodeRemove       = 0xa6
	OutputTypeDomainAccept     = 0xa7
	OutputTypeDomainRemove     = 0xa8
	OutputTypeWithdrawalClaim  = 0xa9

	TransactionTypeScript           = 0x00
	TransactionTypeMint             = 0x01
	TransactionTypeDeposit          = 0x02
	TransactionTypeWithdrawalSubmit = 0x03
	TransactionTypeWithdrawalFuel   = 0x04
	TransactionTypeWithdrawalClaim  = 0x05
	TransactionTypeNodePledge       = 0x06
	TransactionTypeNodeAccept       = 0x07
	TransactionTypeNodeDepart       = 0x08
	TransactionTypeNodeRemove       = 0x09
	TransactionTypeDomainAccept     = 0x10
	TransactionTypeDomainRemove     = 0x11
	TransactionTypeUnknown          = 0xff
)

type Input struct {
	Hash    crypto.Hash  `json:"hash,omitempty"`
	Index   int          `json:"index,omitempty"`
	Genesis []byte       `json:"genesis,omitempty"`
	Deposit *DepositData `json:"deposit,omitempty"`
	Mint    *MintData    `json:"mint,omitempty"`
}

type Output struct {
	Type   uint8        `json:"type"`
	Amount Integer      `json:"amount"`
	Keys   []crypto.Key `json:"keys,omitempty"`

	// OutputTypeScript fields
	Script Script     `json:"script,omitempty"`
	Mask   crypto.Key `json:"mask,omitempty"`
}

type Transaction struct {
	Version uint8       `json:"version"`
	Asset   crypto.Hash `json:"asset"`
	Inputs  []*Input    `json:"inputs"`
	Outputs []*Output   `json:"outputs"`
	Extra   []byte      `json:"extra,omitempty"`
}

type SignedTransaction struct {
	Transaction
	Signatures [][]crypto.Signature `json:"signatures,omitempty"`
}

func (tx *Transaction) ViewGhostKey(a *crypto.Key) []*Output {
	outputs := make([]*Output, 0)

	for i, o := range tx.Outputs {
		if o.Type != OutputTypeScript {
			continue
		}

		out := &Output{
			Type:   o.Type,
			Amount: o.Amount,
			Script: o.Script,
			Mask:   o.Mask,
		}
		for _, k := range o.Keys {
			key := crypto.ViewGhostOutputKey(&k, a, &o.Mask, uint64(i))
			out.Keys = append(out.Keys, *key)
		}
		outputs = append(outputs, out)
	}

	return outputs
}

func (tx *SignedTransaction) CheckMint() bool {
	return len(tx.Inputs) == 1 && tx.Inputs[0].Mint != nil
}

func (tx *SignedTransaction) TransactionType() uint8 {
	for _, in := range tx.Inputs {
		if in.Mint != nil {
			return TransactionTypeMint
		}
		if in.Deposit != nil {
			return TransactionTypeDeposit
		}
		if in.Genesis != nil {
			return TransactionTypeUnknown
		}
	}

	isScript := true
	for _, out := range tx.Outputs {
		switch out.Type {
		case OutputTypeWithdrawalSubmit:
			return TransactionTypeWithdrawalSubmit
		case OutputTypeWithdrawalFuel:
			return TransactionTypeWithdrawalFuel
		case OutputTypeWithdrawalClaim:
			return TransactionTypeWithdrawalClaim
		case OutputTypeNodePledge:
			return TransactionTypeNodePledge
		case OutputTypeNodeAccept:
			return TransactionTypeNodeAccept
		case OutputTypeNodeDepart:
			return TransactionTypeNodeDepart
		case OutputTypeNodeRemove:
			return TransactionTypeNodeRemove
		case OutputTypeDomainAccept:
			return TransactionTypeDomainAccept
		case OutputTypeDomainRemove:
			return TransactionTypeDomainRemove
		}
		isScript = isScript && out.Type == OutputTypeScript
	}

	if isScript {
		return TransactionTypeScript
	}
	return TransactionTypeUnknown
}

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

	var inputAmount, outputAmount Integer

	inputsFilter := make(map[string]*UTXO)
	for i, in := range tx.Inputs {
		if in.Mint != nil {
			inputAmount = in.Mint.Amount
			break
		}

		if in.Deposit != nil {
			inputAmount = in.Deposit.Amount
			break
		}

		fk := fmt.Sprintf("%s:%d", in.Hash.String(), in.Index)
		if inputsFilter[fk] != nil {
			return fmt.Errorf("invalid input %s", fk)
		}

		utxo, err := store.ReadUTXO(in.Hash, in.Index)
		if err != nil {
			return err
		}
		if utxo == nil {
			return fmt.Errorf("input not found %s:%d", in.Hash.String(), in.Index)
		}
		if utxo.Asset != tx.Asset {
			return fmt.Errorf("invalid input asset %s %s", utxo.Asset.String(), tx.Asset.String())
		}

		err = validateUTXO(utxo, tx.Signatures[i], msg)
		if err != nil {
			return err
		}
		inputsFilter[fk] = utxo
		inputAmount = inputAmount.Add(utxo.Amount)
	}

	outputsFilter := make(map[crypto.Key]bool)
	for _, o := range tx.Outputs {
		if o.Amount.Sign() <= 0 {
			return fmt.Errorf("invalid output amount %s", o.Amount.String())
		}
		for _, k := range o.Keys {
			if outputsFilter[k] {
				return fmt.Errorf("invalid output key %s", k.String())
			}
			outputsFilter[k] = true
			exist, err := store.CheckGhost(k)
			if err != nil {
				return err
			} else if exist {
				return fmt.Errorf("invalid output key %s", k.String())
			}
		}
		if o.Type == OutputTypeScript {
			err := o.Script.VerifyFormat()
			if err != nil {
				return err
			}
		}
		outputAmount = outputAmount.Add(o.Amount)
	}

	if inputAmount.Sign() <= 0 || inputAmount.Cmp(outputAmount) != 0 {
		return fmt.Errorf("invalid input output amount %s %s", inputAmount.String(), outputAmount.String())
	}

	switch txType {
	case TransactionTypeScript:
		for _, in := range inputsFilter {
			if in.Type != OutputTypeScript {
				return fmt.Errorf("invalid utxo type %d", in.Type)
			}
		}
	case TransactionTypeMint:
		err := ver.validateMint(store)
		if err != nil {
			return err
		}
	case TransactionTypeDeposit:
		err := tx.validateDeposit(store, msg, ver.PayloadHash())
		if err != nil {
			return err
		}
	case TransactionTypeWithdrawalSubmit:
	case TransactionTypeWithdrawalFuel:
	case TransactionTypeWithdrawalClaim:
	case TransactionTypeNodePledge:
		err := tx.validateNodePledge(store, inputsFilter)
		if err != nil {
			return err
		}
	case TransactionTypeNodeAccept:
		err := tx.validateNodeAccept(store, inputAmount)
		if err != nil {
			return err
		}
	case TransactionTypeNodeDepart:
	case TransactionTypeNodeRemove:
	case TransactionTypeDomainAccept:
	case TransactionTypeDomainRemove:
	}
	return nil
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

func (signed *SignedTransaction) SignInput(reader UTXOReader, index int, accounts []Address) error {
	msg := MsgpackMarshalPanic(signed.Transaction)

	if len(accounts) == 0 {
		return nil
	}
	if index >= len(signed.Inputs) {
		return fmt.Errorf("invalid input index %d/%d", index, len(signed.Inputs))
	}
	in := signed.Inputs[index]
	if in.Deposit != nil || in.Mint != nil {
		return signed.SignRaw(accounts[0].PrivateSpendKey)
	}

	utxo, err := reader.ReadUTXO(in.Hash, in.Index)
	if err != nil {
		return err
	}
	if utxo == nil {
		return fmt.Errorf("input not found %s:%d", in.Hash.String(), in.Index)
	}

	keysFilter := make(map[string]bool)
	for _, k := range utxo.Keys {
		keysFilter[k.String()] = true
	}

	sigs := make([]crypto.Signature, 0)
	for _, acc := range accounts {
		priv := crypto.DeriveGhostPrivateKey(&utxo.Mask, &acc.PrivateViewKey, &acc.PrivateSpendKey, uint64(in.Index))
		if keysFilter[priv.Public().String()] {
			sigs = append(sigs, priv.Sign(msg))
		}
	}
	signed.Signatures = append(signed.Signatures, sigs)
	return nil
}

func (signed *SignedTransaction) SignRaw(key crypto.Key) error {
	msg := MsgpackMarshalPanic(signed.Transaction)

	if len(signed.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d", len(signed.Inputs))
	}
	in := signed.Inputs[0]
	if in.Deposit == nil && in.Mint == nil {
		return fmt.Errorf("invalid input format")
	}
	if in.Deposit != nil {
		err := signed.verifyDepositFormat()
		if err != nil {
			return err
		}
	}
	signed.Signatures = append(signed.Signatures, []crypto.Signature{key.Sign(msg)})
	return nil
}

func NewTransaction(asset crypto.Hash) *Transaction {
	return &Transaction{
		Version: TxVersion,
		Asset:   asset,
	}
}

func (tx *Transaction) AddInput(hash crypto.Hash, index int) {
	in := &Input{
		Hash:  hash,
		Index: index,
	}
	tx.Inputs = append(tx.Inputs, in)
}

func (tx *Transaction) AddOutputWithType(ot uint8, accounts []Address, s Script, amount Integer, seed []byte) {
	r := crypto.NewKeyFromSeed(seed)
	R := r.Public()
	out := &Output{
		Type:   ot,
		Amount: amount,
		Script: s,
		Mask:   R,
		Keys:   make([]crypto.Key, 0),
	}

	for _, a := range accounts {
		k := crypto.DeriveGhostPublicKey(&r, &a.PublicViewKey, &a.PublicSpendKey, uint64(len(tx.Outputs)))
		out.Keys = append(out.Keys, *k)
	}
	tx.Outputs = append(tx.Outputs, out)
}

func (tx *Transaction) AddScriptOutput(accounts []Address, s Script, amount Integer, seed []byte) {
	tx.AddOutputWithType(OutputTypeScript, accounts, s, amount, seed)
}

func (tx *Transaction) AddRandomScriptOutput(accounts []Address, s Script, amount Integer) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	tx.AddScriptOutput(accounts, s, amount, seed)
	return nil
}
