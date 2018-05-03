package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/NebulousLabs/Sia/crypto"
	"github.com/NebulousLabs/Sia/types"
)

func LocalNormalizePublicKey(address string) (string, error) {
	var uh types.UnlockHash
	err := uh.LoadString(address)
	if err != nil {
		return "", err
	}
	if uh.String() != address {
		return "", fmt.Errorf("Siacoin NormalizeAddress mismatch %s", address)
	}
	return uh.String(), nil
}

func LocalGenerateKey() (string, string, error) {
	var seed [crypto.EntropySize]byte
	_, err := rand.Read(seed[:])
	if err != nil {
		return "", "", err
	}
	_, pk := crypto.GenerateKeyPairDeterministic(seed)
	priv := hex.EncodeToString(seed[:])
	pub := types.UnlockConditions{
		PublicKeys:         []types.SiaPublicKey{types.Ed25519PublicKey(pk)},
		SignaturesRequired: 1,
	}.UnlockHash().String()
	return pub, priv, nil
}
