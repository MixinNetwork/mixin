# Mixin Inscription

This proposal defines a protocol to issue tokens and collectibles by embedding data in Mixin Kernel storage transaction.

Mixin Kernel is a distributed ledger with UTXO model, and an inscription will create a unique UTXO, which can represent both a collectible object and some fungible tokens at the same time.

A Mixin inscription in this protocol should follow the specification so that the wallets and other applications could display correctly of the inscriptions.

## Deploy

An inscription in Mixin Kernel must belong to a collection, and a collection must begin with a deployment transaction. The deployment transaction just need to include the following JSON as extra, and due to the kernel limit, the maximum extra size is 4MB.

```golang
type Deployment struct {
	// the version must be 1
	Version uint8 `json:"version"`

	// operation must be deploy
	Operation string `json:"operation"`

	// 1 distribute tokens per inscription
	// 2 distribute tokens after inscription progress done
	Mode uint8 `json:"mode"`

	// supply is the total supply of all tokens
	// unit is the amount of tokens per inscription
	//
	// if supply is 1,000,000,000 and unit is 1,000,000,
	// then there should be 1,000 inscription operations
	// 1 inscription represents 1 collectible, so there will be 1,000 NFTs
	Unit   string `json:"unit"`
	Supply string `json:"supply"`

	// the token symbol and name are required and must be valid UTF8
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// the icon must be in valid data URI scheme
	// e.g. image/webp;base64,IVVB===
	Icon string `json:"icon"`

	// only needed if the deployer wants to limit the contents of inscriptions
	// there are only two validation rules supported:
	// CHECKSUM: base64 of all NFT blake3 checksums, and all checksums
	// must be different from each other.
	// REGEX: a valid regex for text only content
	Validation string `json:"validation,omitempty"`

	// ratio of each inscribed tokens will be kept in treasury
	// the treasury tokens will be distributed to the recipient MIX address
	// at the same time  as defined by the mode
	//
	// For MAO, the ratio will be 0.9, and each collectible will only cost
	// 10% of the unit tokens, so only the inscribers have NFTs, but not
	// the treasury tokens, however they can occupy a vacant NFT.
	Treasury *struct {
		Ratio     string `json:"ratio"`
		Recipient string `json:"recipient"`
	} `json:"treasury,omitempty"`
}
```

## Inscribe

Once an inscription collection is deployed, then anyone can inscribe by including the following JSON as extra. The inscription transaction must reference the collection transaction hash to indicate which collection to inscribe.

If the deployer included checksum in the collection deployment, then each inscription content must be valid, otherwise this inscription will be ignored by the protocol. A typical NFT collection with fixed supply is recommended to use this approach.

If no checksum included, then anyone can include any content in the inscription or even leave it as empty. If the content is not empty, then the first occurrence is valid and the following inscription with the same content will be ignored by the protocol. Unlimited NFT such as domain names, or just fungible tokens without specific NFT purpose could use this approach.

```golang
type Inscription struct {
	// operation must be inscribe
	Operation string `json:"operation"`

	// Recipient can only be MIX address, not ghost keys, because
	// otherwise the keys may be used by others, then redeemed invalid
	Recipient string `json:"recipient"`

	// data URI scheme
	// application/octet-stream;key=fingerprint;base64,iVBO==
	// image/webp;trait=one;base64,iVBO==
	// text/plain;charset=UTF-8,cedric.mao
	// text/plain;charset=UTF-8;base64,iii==
	Content string `json:"content,omitempty"`
}
```

## Distribute

According to the rule of the collection mode, a distributor needs to distribute a UTXO to the recipient per inscription, optionally with a UTXO for treasury.

The distribution transaction must reference the inscription transaction and include the following JSON extra.

```golang
type Distribution struct {
	// operation must be distribute
	Operation string `json:"distribute"`

	// sequence must monotonically increase from 0
	Sequence uint64 `json:"sequence"`
}
```

## Transfer

There is no special protocol for inscription token transfers. However the senders should be careful to not split or combine an NFT UTXO if they don't want to release the NFT.

## Release

For an NFT inscription, the owner will release the NFT if they split or combine the UTXO. After an NFT is released, it can be occupied by the unit amount of tokens.

## Occupy

Whoever receives a transaction with the exact unit amount of inscription tokens at the first UTXO will occupy the vacant NFT.

The occupancy transaction must reference the NFT initial inscription transaction hash and include the following JSON extra.

```golang
type Occupation struct {
	// operation must be occupy
	Operation string `json:"operation"`

	// the integer sequence number of the NFT inscription
	Sequence uint64 `json:"sequence"`
}
```
