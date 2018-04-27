package api

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/rubblelabs/ripple/crypto"
	"github.com/rubblelabs/ripple/data"
)

func LocalNormalizePublicKey(address string) (string, error) {
	destinationHash, err := crypto.NewRippleHash(address)
	if err != nil {
		return "", fmt.Errorf("Ripple NormalizeAddress hash error %s", err.Error())
	}
	var destination data.Account
	if len(destinationHash.Payload()) != len(destination) {
		return "", fmt.Errorf("Ripple NormalizeAddress invalid %s", address)
	}
	accountId, err := crypto.NewAccountId(destinationHash.Payload())
	if err != nil {
		return "", fmt.Errorf("Ripple NormalizeAddress account error %s", err.Error())
	}
	if accountId.String() != address {
		return "", fmt.Errorf("Ripple NormalizeAddress mismatch %s", address)
	}
	return accountId.String(), nil
}

func LocalGenerateKey() (string, string, error) {
	key, err := crypto.NewECDSAKey(nil)
	if err != nil {
		return "", "", err
	}
	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), key.PrivateKey.Serialize())
	accountId, err := crypto.NewAccountId(crypto.Sha256RipeMD160(publicKey.SerializeCompressed()))
	if err != nil {
		return "", "", err
	}
	return accountId.String(), hex.EncodeToString(privateKey.Serialize()), nil
}
