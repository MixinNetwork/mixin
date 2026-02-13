module github.com/MixinNetwork/mixin

go 1.26.0

replace github.com/dgraph-io/badger/v4 => github.com/MixinNetwork/badger/v4 v4.9.0-F1

require (
	filippo.io/edwards25519 v1.1.0
	github.com/dgraph-io/badger/v4 v4.9.0
	github.com/dgraph-io/ristretto/v2 v2.4.0
	github.com/pelletier/go-toml v1.9.5
	github.com/quic-go/quic-go v0.59.0
	github.com/shopspring/decimal v1.4.0
	github.com/stretchr/testify v1.11.1
	github.com/urfave/cli/v2 v2.27.7
	github.com/zeebo/blake3 v0.2.4
	golang.org/x/crypto v0.47.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/klauspost/compress v1.18.3 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20250705151800-55b8f293f342 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
