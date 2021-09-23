module github.com/MixinNetwork/mixin

go 1.17

replace github.com/vmihailenco/msgpack/v4 => github.com/MixinNetwork/msgpack/v4 v4.3.13

require (
	filippo.io/edwards25519 v1.0.0-rc.1
	github.com/MixinNetwork/mobilecoin-go v0.0.0-20210622080237-fed5245fb80c
	github.com/VictoriaMetrics/fastcache v1.7.0
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/cpacia/bchutil v0.0.0-20181003130114-b126f6a35b6c
	github.com/decred/dcrd/dcrutil v1.4.0
	github.com/dgraph-io/badger/v3 v3.2103.1
	github.com/filecoin-project/go-address v0.0.6
	github.com/filecoin-project/go-state-types v0.1.0
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/ipfs/go-cid v0.1.0
	github.com/klauspost/compress v1.13.6
	github.com/lucas-clemente/quic-go v0.23.0
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/multiformats/go-multihash v0.0.16
	github.com/paxosglobal/moneroutil v0.0.0-20170611151923-33d7e0c11a62
	github.com/pelletier/go-toml v1.9.4
	github.com/shopspring/decimal v1.2.0
	github.com/stellar/go v0.0.0-20210922145424-856a1aba5220
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli/v2 v2.3.0
	github.com/vmihailenco/msgpack/v4 v4.3.12
	github.com/willf/bitset v1.1.11
	go.dedis.ch/kyber/v3 v3.0.13
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/bwesterb/go-ristretto v1.1.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cheekybits/genny v1.0.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0-20190314233015-f79a8a8ca69d // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dchest/blake256 v1.0.0 // indirect
	github.com/dchest/blake2b v1.0.0 // indirect
	github.com/decred/base58 v1.0.0 // indirect
	github.com/decred/dcrd/chaincfg v1.5.1 // indirect
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1 // indirect
	github.com/decred/dcrd/chaincfg/v2 v2.0.2 // indirect
	github.com/decred/dcrd/dcrec v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/edwards v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1 v1.0.2 // indirect
	github.com/decred/dcrd/dcrutil/v2 v2.0.0 // indirect
	github.com/decred/dcrd/wire v1.2.0 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/ebfe/keccak v0.0.0-20150115210727-5cc570678d1b // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v1.12.0 // indirect
	github.com/gtank/merlin v0.1.1 // indirect
	github.com/ipfs/go-block-format v0.0.2 // indirect
	github.com/ipfs/go-ipfs-util v0.0.1 // indirect
	github.com/ipfs/go-ipld-cbor v0.0.5 // indirect
	github.com/ipfs/go-ipld-format v0.0.1 // indirect
	github.com/klauspost/cpuid/v2 v2.0.4 // indirect
	github.com/marten-seemann/qtls-go1-16 v0.1.4 // indirect
	github.com/marten-seemann/qtls-go1-17 v0.1.0 // indirect
	github.com/mimoo/StrobeGo v0.0.0-20181016162300-f8f6d4d2b643 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.0.3 // indirect
	github.com/multiformats/go-base36 v0.1.0 // indirect
	github.com/multiformats/go-multibase v0.0.3 // indirect
	github.com/multiformats/go-varint v0.0.6 // indirect
	github.com/nwaples/rardecode v1.1.2 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.4 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polydawn/refmt v0.0.0-20190807091052-3d65705ee9f1 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/stellar/go-xdr v0.0.0-20201028102745-f80a23dac78a // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/vmihailenco/tagparser v0.1.1 // indirect
	github.com/whyrusleeping/cbor-gen v0.0.0-20210303213153-67a261a1d291 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	go.dedis.ch/fixbuf v1.0.3 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210910150752-751e447fb3d0 // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/tools v0.1.4 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210624195500-8bfb893ecb84 // indirect
	google.golang.org/grpc v1.38.0 // indirect
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)
