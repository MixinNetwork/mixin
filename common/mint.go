package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MintGroupUniversal = "UNIVERSAL"
)

type MintData struct {
	Group  string
	Batch  uint64
	Amount Integer
}

type MintDistribution struct {
	MintData
	Transaction crypto.Hash
}

func (m *MintData) Distribute(tx crypto.Hash) *MintDistribution {
	return &MintDistribution{
		MintData:    *m,
		Transaction: tx,
	}
}

func (tx *VersionedTransaction) validateMint(store DataStore) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for mint", len(tx.Inputs))
	}
	for _, out := range tx.Outputs {
		if out.Type != OutputTypeScript {
			return fmt.Errorf("invalid mint output type %d", out.Type)
		}
	}
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid mint asset %s", tx.Asset.String())
	}

	mint := tx.Inputs[0].Mint
	if mint.Group != MintGroupUniversal {
		return fmt.Errorf("invalid mint group %s", mint.Group)
	}

	dist, err := store.ReadLastMintDistribution(mint.Group)
	if err != nil {
		return err
	}
	if mint.Batch > dist.Batch {
		return nil
	}
	if mint.Batch < dist.Batch {
		return fmt.Errorf("backward mint batch %d %d", dist.Batch, mint.Batch)
	}
	if dist.Transaction != tx.PayloadHash() || dist.Amount.Cmp(mint.Amount) != 0 {
		return fmt.Errorf("invalid mint lock %s %s", dist.Transaction.String(), tx.PayloadHash().String())
	}
	return nil
}

func (tx *Transaction) AddKernelNodeMintInput(batch uint64, amount Integer) {
	tx.Inputs = append(tx.Inputs, &Input{
		Mint: &MintData{
			Group:  MintGroupUniversal,
			Batch:  batch,
			Amount: amount,
		},
	})
}

func (m *MintDistribution) CompressMarshal() []byte {
	return compress(m.Marshal())
}

func DecompressUnmarshalMintDistribution(b []byte) (*MintDistribution, error) {
	d := decompress(b)
	if d == nil {
		d = b
	}
	return UnmarshalMintDistribution(d)
}

func (m *MintDistribution) Marshal() []byte {
	enc := NewMinimumEncoder()
	switch m.Group {
	case MintGroupUniversal:
		enc.WriteUint16(0x1)
	default:
		panic(m.Group)
	}
	enc.WriteUint64(m.Batch)
	enc.WriteInteger(m.Amount)
	enc.Write(m.Transaction[:])
	return enc.Bytes()
}

func UnmarshalMintDistribution(b []byte) (*MintDistribution, error) {
	if len(b) < 16 {
		return nil, fmt.Errorf("invalid mint distribution size %d", len(b))
	}

	var m MintDistribution
	dec, err := NewMinimumDecoder(b)
	if err != nil {
		err := msgpackUnmarshal(b, &m)
		return &m, err
	}

	group, err := dec.ReadUint16()
	if err != nil {
		return nil, err
	}
	switch group {
	case 0x1:
		m.Group = MintGroupUniversal
	default:
		return nil, fmt.Errorf("invalid mint distribution group %d", group)
	}

	batch, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	m.Batch = batch

	amount, err := dec.ReadInteger()
	if err != nil {
		return nil, err
	}
	m.Amount = amount

	err = dec.Read(m.Transaction[:])
	return &m, err
}
