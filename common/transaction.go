package common

import (
	"crypto/rand"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	TxVersion       = 0x02
	ExtraSizeLimit  = 256
	SliceCountLimit = 256

	OutputTypeScript             = 0x00
	OutputTypeWithdrawalSubmit   = 0xa1
	OutputTypeWithdrawalFuel     = 0xa2
	OutputTypeNodePledge         = 0xa3
	OutputTypeNodeAccept         = 0xa4
	outputTypeNodeResign         = 0xa5
	OutputTypeNodeRemove         = 0xa6
	OutputTypeDomainAccept       = 0xa7
	OutputTypeDomainRemove       = 0xa8
	OutputTypeWithdrawalClaim    = 0xa9
	OutputTypeNodeCancel         = 0xaa
	OutputTypeDomainAssetCustody = 0xab
	OutputTypeDomainAssetRelease = 0xac
	OutputTypeDomainAssetMigrate = 0xad

	TransactionTypeScript           = 0x00
	TransactionTypeMint             = 0x01
	TransactionTypeDeposit          = 0x02
	TransactionTypeWithdrawalSubmit = 0x03
	TransactionTypeWithdrawalFuel   = 0x04
	TransactionTypeWithdrawalClaim  = 0x05
	TransactionTypeNodePledge       = 0x06
	TransactionTypeNodeAccept       = 0x07
	transactionTypeNodeResign       = 0x08
	TransactionTypeNodeRemove       = 0x09
	TransactionTypeDomainAccept     = 0x10
	TransactionTypeDomainRemove     = 0x11
	TransactionTypeNodeCancel       = 0x12
	TransactionTypeUnknown          = 0xff
)

type Input struct {
	Hash    crypto.Hash
	Index   int
	Genesis []byte
	Deposit *DepositData
	Mint    *MintData
}

type Output struct {
	Type       uint8
	Amount     Integer
	Keys       []crypto.Key
	Withdrawal *WithdrawalData `msgpack:",omitempty"`

	// OutputTypeScript fields
	Script Script
	Mask   crypto.Key
}

type Transaction struct {
	Version uint8
	Asset   crypto.Hash
	Inputs  []*Input
	Outputs []*Output
	Extra   []byte
}

type SignedTransaction struct {
	Transaction
	AggregatedSignature *AggregatedSignature           `msgpack:"-"`
	SignaturesMap       []map[uint16]*crypto.Signature `msgpack:"Signatures"`
	SignaturesSliceV1   [][]*crypto.Signature          `msgpack:"-"`
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
		case OutputTypeNodeCancel:
			return TransactionTypeNodeCancel
		case OutputTypeNodeAccept:
			return TransactionTypeNodeAccept
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

func (signed *SignedTransaction) SignUTXO(utxo *UTXO, accounts []*Address) error {
	msg := signed.AsLatestVersion().PayloadMarshal()

	if len(accounts) == 0 {
		return nil
	}

	keysFilter := make(map[string]uint16)
	for i, k := range utxo.Keys {
		keysFilter[k.String()] = uint16(i)
	}

	sigs := make(map[uint16]*crypto.Signature)
	for _, acc := range accounts {
		priv := crypto.DeriveGhostPrivateKey(&utxo.Mask, &acc.PrivateViewKey, &acc.PrivateSpendKey, uint64(utxo.Index))
		i, found := keysFilter[priv.Public().String()]
		if !found {
			return fmt.Errorf("invalid key for the input %s", acc.String())
		}
		sig := priv.Sign(msg)
		sigs[i] = &sig
	}
	signed.SignaturesMap = append(signed.SignaturesMap, sigs)
	return nil
}

func (signed *SignedTransaction) SignInput(reader UTXOReader, index int, accounts []*Address) error {
	msg := signed.AsLatestVersion().PayloadMarshal()

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

	keysFilter := make(map[string]uint16)
	for i, k := range utxo.Keys {
		keysFilter[k.String()] = uint16(i)
	}

	sigs := make(map[uint16]*crypto.Signature)
	for _, acc := range accounts {
		priv := crypto.DeriveGhostPrivateKey(&utxo.Mask, &acc.PrivateViewKey, &acc.PrivateSpendKey, uint64(in.Index))
		i, found := keysFilter[priv.Public().String()]
		if !found {
			return fmt.Errorf("invalid key for the input %s", acc.String())
		}
		sig := priv.Sign(msg)
		sigs[i] = &sig
	}
	signed.SignaturesMap = append(signed.SignaturesMap, sigs)
	return nil
}

func (signed *SignedTransaction) SignRaw(key crypto.Key) error {
	msg := signed.AsLatestVersion().PayloadMarshal()

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
	sig := key.Sign(msg)
	sigs := map[uint16]*crypto.Signature{0: &sig}
	signed.SignaturesMap = append(signed.SignaturesMap, sigs)
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

func (tx *Transaction) AddOutputWithType(ot uint8, accounts []*Address, s Script, amount Integer, seed []byte) {
	out := &Output{
		Type:   ot,
		Amount: amount,
		Script: s,
		Keys:   make([]crypto.Key, 0),
	}

	if len(accounts) > 0 {
		r := crypto.NewKeyFromSeed(seed)
		out.Mask = r.Public()
		for _, a := range accounts {
			k := crypto.DeriveGhostPublicKey(&r, &a.PublicViewKey, &a.PublicSpendKey, uint64(len(tx.Outputs)))
			out.Keys = append(out.Keys, *k)
		}
	}

	tx.Outputs = append(tx.Outputs, out)
}

func (tx *Transaction) AddScriptOutput(accounts []*Address, s Script, amount Integer, seed []byte) {
	tx.AddOutputWithType(OutputTypeScript, accounts, s, amount, seed)
}

func (tx *Transaction) AddRandomScriptOutput(accounts []*Address, s Script, amount Integer) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	tx.AddScriptOutput(accounts, s, amount, seed)
	return nil
}
