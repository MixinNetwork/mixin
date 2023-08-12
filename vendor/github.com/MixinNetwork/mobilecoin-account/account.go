package api

import (
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"

	"github.com/MixinNetwork/mobilecoin-account/base58"
	"github.com/MixinNetwork/mobilecoin-account/types"
	"github.com/bwesterb/go-ristretto"
	"github.com/dchest/blake2b"
	"google.golang.org/protobuf/proto"
)

const (
	SUBADDRESS_DOMAIN_TAG = "mc_subaddress"
)

type Account struct {
	ViewPrivateKey  *ristretto.Scalar
	SpendPrivateKey *ristretto.Scalar
}

func NewAccountKey(viewPrivate, spendPrivate string) (*Account, error) {
	account := &Account{
		ViewPrivateKey: HexToScalar(viewPrivate),
	}
	if spendPrivate != "" {
		account.SpendPrivateKey = HexToScalar(spendPrivate)
	}
	return account, nil
}

func (account *Account) SubaddressSpendPrivateKey(index uint64) *ristretto.Scalar {
	var buf [32]byte
	binary.LittleEndian.PutUint64(buf[:], index)
	var address ristretto.Scalar
	hash := blake2b.New512()
	hash.Write([]byte(SUBADDRESS_DOMAIN_TAG))
	hash.Write(account.ViewPrivateKey.Bytes())
	hash.Write(address.SetBytes(&buf).Bytes())

	var hs ristretto.Scalar
	var key [64]byte
	copy(key[:], hash.Sum(nil))

	var private ristretto.Scalar
	return private.Add(hs.SetReduced(&key), account.SpendPrivateKey)
}

func (account *Account) SubaddressViewPrivateKey(spend *ristretto.Scalar) *ristretto.Scalar {
	var private ristretto.Scalar
	return private.Mul(account.ViewPrivateKey, spend)
}

func (account *Account) PublicAddress(index uint64) *PublicAddress {
	spendPrivate := account.SubaddressSpendPrivateKey(index)
	spend := PublicKey(spendPrivate)
	view := PublicKey(account.SubaddressViewPrivateKey(spendPrivate))

	return &PublicAddress{
		ViewPublicKey:  hex.EncodeToString(view.Bytes()),
		SpendPublicKey: hex.EncodeToString(spend.Bytes()),
	}
}

func (account *Account) B58Code(index uint64) (string, error) {
	spendPrivate := account.SubaddressSpendPrivateKey(index)
	spend := PublicKey(spendPrivate)
	view := PublicKey(account.SubaddressViewPrivateKey(spendPrivate))

	address := &types.PublicAddress{
		ViewPublicKey:  &types.CompressedRistretto{Data: view.Bytes()},
		SpendPublicKey: &types.CompressedRistretto{Data: spend.Bytes()},
	}
	wrapper := &types.PrintableWrapper_PublicAddress{PublicAddress: address}
	data, err := proto.Marshal(&types.PrintableWrapper{Wrapper: wrapper})
	if err != nil {
		return "", err
	}

	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, crc32.ChecksumIEEE(data))
	bytes = append(bytes, data...)
	return base58.Encode(bytes), nil
}

func B58CodeFromSpend(viewPrivate string, spend *ristretto.Scalar) (string, error) {
	account, err := NewAccountKey(viewPrivate, "")
	if err != nil {
		return "", err
	}
	view := PublicKey(account.SubaddressViewPrivateKey(spend))

	address := &types.PublicAddress{
		ViewPublicKey: &types.CompressedRistretto{
			Data: view.Bytes(),
		},
		SpendPublicKey: &types.CompressedRistretto{
			Data: spend.Bytes(),
		},
	}
	wrapper := &types.PrintableWrapper_PublicAddress{PublicAddress: address}
	data, err := proto.Marshal(&types.PrintableWrapper{Wrapper: wrapper})
	if err != nil {
		return "", err
	}

	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, crc32.ChecksumIEEE(data))
	bytes = append(bytes, data...)
	return base58.Encode(bytes), nil
}

func PublicKey(private *ristretto.Scalar) *ristretto.Point {
	var point ristretto.Point
	return point.ScalarMultBase(private)
}

func SharedSecret(viewPrivate, publicKey string) *ristretto.Point {
	public := HexToPoint(publicKey)
	private := HexToScalar(viewPrivate)
	return CreateSharedSecret(public, private)
}

func CreateSharedSecret(public *ristretto.Point, private *ristretto.Scalar) *ristretto.Point {
	var r ristretto.Point
	return r.ScalarMult(public, private)
}

func HexToPoint(h string) *ristretto.Point {
	buf := HexToBytes(h)
	var buf32 [32]byte
	copy(buf32[:], buf)
	var s ristretto.Point
	s.SetBytes(&buf32)
	return &s
}

func HexToScalar(h string) *ristretto.Scalar {
	buf := HexToBytes(h)
	var buf32 [32]byte
	copy(buf32[:], buf)
	var s ristretto.Scalar
	return s.SetBytes(&buf32)
}

func HexToBytes(h string) []byte {
	buf, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return buf
}
