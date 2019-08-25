module github.com/MixinNetwork/mixin

go 1.12

replace github.com/dgraph-io/badger => github.com/MixinNetwork/badger v0.0.0-20190825150528-5fefce7e318c

require (
	github.com/MixinNetwork/msgpack v4.0.5+incompatible
	github.com/VictoriaMetrics/fastcache v1.5.1
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/dgraph-io/badger v2.0.0-rc2+incompatible
	github.com/dimfeld/httptreemux v5.0.1+incompatible
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/ethereum/go-ethereum v1.9.2
	github.com/gobuffalo/packr v1.30.1
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/gorilla/handlers v1.4.2
	github.com/lucas-clemente/quic-go v0.11.2
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/nwaples/rardecode v1.0.0 // indirect
	github.com/pierrec/lz4 v2.2.6+incompatible // indirect
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24
	github.com/stretchr/testify v1.4.0
	github.com/unrolled/render v1.0.1
	github.com/urfave/cli v1.21.0
	github.com/valyala/gozstd v1.6.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	go.dedis.ch/kyber/v3 v3.0.4
	golang.org/x/crypto v0.0.0-20190621222207-cc06ce4a13d4
)
