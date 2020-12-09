package eos

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/gofrs/uuid"
)

var (
	EOSChainBase string
	EOSChainId   crypto.Hash
)

func init() {
	EOSChainBase = "6cfe566e-4aad-470b-8c9a-2fd35b49c68d"
	EOSChainId = crypto.NewHash([]byte(EOSChainBase))
}

func VerifyAssetKey(assetKey string) error {
	parts := strings.Split(assetKey, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid eos asset key %s", assetKey)
	}
	account, symbol := parts[0], parts[1]
	err := VerifyAddress(account)
	if err != nil {
		return err
	}
	if len(symbol) > 8 {
		return fmt.Errorf("invalid eos asset key %s", assetKey)
	}
	return nil
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid eos address %s", address)
	}
	if strings.ToLower(address) != address {
		return fmt.Errorf("invalid eos address %s", address)
	}
	dot := strings.Index(address, ".")
	if dot == 0 || dot == len(address)-1 {
		return fmt.Errorf("invalid eos address %s", address)
	}
	if dot > 0 {
		address = address[:dot] + address[dot+1:]
	}
	dot = strings.Index(address, ".")
	if dot >= 0 {
		return fmt.Errorf("invalid eos address %s", address)
	}
	if len(address) > 12 {
		return fmt.Errorf("invalid eos address %s", address)
	}
	matched, err := regexp.MatchString("^[a-z1-5]{1,12}$", address)
	if err != nil || !matched {
		return fmt.Errorf("invalid eos address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid eos transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid eos transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid eos transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid eos transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == "eosio.token:EOS" {
		return EOSChainId
	}

	h := md5.New()
	io.WriteString(h, EOSChainBase)
	io.WriteString(h, assetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}
