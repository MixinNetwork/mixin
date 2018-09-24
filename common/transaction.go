package common

import (
	"crypto/rand"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	TxVersion      = 0x01
	ExtraSizeLimit = 256

	InputTypeScript  = 0x00
	InputTypeDeposit = 0x71
	InputTypeRebate  = 0x72
	InputTypeMint    = 0x73

	OutputTypeScript     = 0x00
	OutputTypeWithdrawal = 0xa1
	OutputTypeSlash      = 0xa2
	OutputTypePledge     = 0xa3
	OutputTypeReclaim    = 0xa4
)

type Input struct {
	Hash  crypto.Hash `msgpack:"H"json:"hash"`
	Index int         `msgpack:"I"json:"index"`
}

type Output struct {
	Type   uint8   `msgpack:"T"json:"type"`
	Amount Integer `msgpack:"A"json:"amount"`

	// OutputTypeScript fields
	Script Script       `msgpack:"S,omitempty"json:"script,omitempty"`
	Keys   []crypto.Key `msgpack:"K,omitempty"json:"keys,omitempty"`
	Mask   crypto.Key   `msgpack:"M,omitempty"json:"mask,omitempty"`
}

type Transaction struct {
	Version uint8       `msgpack:"V"json:"version"`
	Asset   crypto.Hash `msgpack:"C"json:"asset"`
	Inputs  []*Input    `msgpack:"I"json:"inputs"`
	Outputs []*Output   `msgpack:"O"json:"outputs"`
	Extra   []byte      `msgpack:"E,omitempty"json:"extra,omitempty"`
}

type SignedTransaction struct {
	Transaction
	Signatures [][]crypto.Signature `msgpack:"S,omitempty"json:"signatures,omitempty"`
}

func (tx *Transaction) ViewGhostKey(a *crypto.Key) []*Output {
	outputs := make([]*Output, 0)

	for _, o := range tx.Outputs {
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
			key := crypto.ViewGhostOutputKey(&k, a, &o.Mask)
			out.Keys = append(out.Keys, *key)
		}
		outputs = append(outputs, out)
	}

	return outputs
}

func (tx *SignedTransaction) Validate(getUTXO UTXOStore, getKey KeyStore) error {
	if tx.Version != TxVersion {
		return fmt.Errorf("invalid tx version %d", tx.Version)
	}

	if len(tx.Inputs) != len(tx.Signatures) {
		return fmt.Errorf("invalid tx signature number %d %d", len(tx.Inputs), len(tx.Signatures))
	}

	if len(tx.Extra) > ExtraSizeLimit {
		return fmt.Errorf("invalid extra size %d", len(tx.Extra))
	}

	msg := MsgpackMarshalPanic(tx.Transaction)

	var input, output Integer
	for _, o := range tx.Outputs {
		for _, k := range o.Keys {
			exist, err := getKey(k)
			if err != nil {
				return err
			} else if exist {
				return fmt.Errorf("invalid output key %s", k.String())
			}
		}
		output = output.Add(o.Amount)
	}

	inputsFilter := make(map[string]bool)

	for i, in := range tx.Inputs {
		fk := fmt.Sprintf("%s:%d", in.Hash.String(), in.Index)
		if inputsFilter[fk] {
			return fmt.Errorf("invalid input %s", fk)
		}
		inputsFilter[fk] = true

		utxo, err := getUTXO(in.Hash, in.Index)
		if err != nil {
			return err
		}
		if utxo.Asset.String() != tx.Asset.String() {
			return fmt.Errorf("invalid input asset %s %s", utxo.Asset.String(), tx.Asset.String())
		}

		err = validateUTXO(utxo, tx.Signatures[i], msg)
		if err != nil {
			return err
		}
		input = input.Add(utxo.Amount)
	}

	if input.Cmp(output) != 0 {
		return fmt.Errorf("invalid input output amount %s %s", input.String(), output.String())
	}

	return nil
}

func validateUTXO(utxo *UTXO, sigs []crypto.Signature, msg []byte) error {
	if utxo.Type != InputTypeScript {
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

func (tx *Transaction) Hash() crypto.Hash {
	msg := MsgpackMarshalPanic(tx)
	return crypto.NewHash(msg)
}

func (tx *SignedTransaction) Marshal() []byte {
	return MsgpackMarshalPanic(tx)
}

func (signed *SignedTransaction) SignInput(getUTXO UTXOStore, index int, accounts []Address) error {
	msg := MsgpackMarshalPanic(signed.Transaction)

	if index >= len(signed.Inputs) {
		return fmt.Errorf("invalid input index %d/%d", index, len(signed.Inputs))
	}
	in := signed.Inputs[index]
	utxo, err := getUTXO(in.Hash, in.Index)
	if err != nil {
		return err
	}

	keysFilter := make(map[string]bool)
	for _, k := range utxo.Keys {
		keysFilter[k.String()] = true
	}

	sigs := make([]crypto.Signature, 0)
	for _, acc := range accounts {
		priv := crypto.DeriveGhostPrivateKey(&utxo.Mask, &acc.PrivateViewKey, &acc.PrivateSpendKey)
		if keysFilter[priv.Public().String()] {
			sigs = append(sigs, priv.Sign(msg))
		}
	}
	signed.Signatures = append(signed.Signatures, sigs)
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

func (tx *Transaction) AddScriptOutput(accounts []Address, s Script, amount Integer) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	r := crypto.NewKeyFromSeed(seed)
	R := r.Public()
	out := &Output{
		Type:   OutputTypeScript,
		Amount: amount,
		Script: s,
		Mask:   R,
		Keys:   make([]crypto.Key, 0),
	}

	for _, a := range accounts {
		k := crypto.DeriveGhostPublicKey(&r, &a.PublicViewKey, &a.PublicSpendKey)
		out.Keys = append(out.Keys, *k)
	}
	tx.Outputs = append(tx.Outputs, out)
	return nil
}
