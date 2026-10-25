module github.com/MixinNetwork/mixin

go 1.27.3

replace github.com/dgraph-io/badger/v4 => github.com/MixinNetwork/badger/v4 v4.9.6-F1

require (
	filippo.io/edwards25519 v1.2.0
	github.com/dgraph-io/badger/v4 v4.9.6
	github.com/dgraph-io/ristretto/v2 v2.4.2
	github.com/pelletier/go-toml v1.9.5
	github.com/quic-go/quic-go v0.64.1
	github.com/shopspring/decimal v1.6.0
	github.com/stretchr/testify v1.12.1
	github.com/urfave/cli/v2 v2.27.7
	github.com/zeebo/blake3 v0.2.4
	golang.org/x/crypto v0.58.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/klauspost/cpuid/v2 v2.4.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20261013231157-39c47d93efba // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
