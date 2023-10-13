package main

import (
	"fmt"
	"os"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func main() {
	buildSnapshots()
}

// zstd --train /tmp/mixin-zstd-snapshots/* -o snapshot.zstd
func buildSnapshots() {
	dir := "/tmp/mixin-zstd-snapshots"
	for n := 0; n < 30; n++ {
		for i := 0; i < 300; i++ {
			tx := crypto.Blake3Hash([]byte(fmt.Sprintf("TRANSACTION:0:%d", time.Now().UnixNano())))
			self := crypto.Blake3Hash([]byte(fmt.Sprintf("REFERENCE:0:%d", time.Now().UnixNano())))
			external := crypto.Blake3Hash([]byte(fmt.Sprintf("REFERENCE:1:%d", time.Now().UnixNano())))
			sh0 := crypto.Blake3Hash([]byte(fmt.Sprintf("SIGNATURE:0:%d", time.Now().UnixNano())))
			sh1 := crypto.Blake3Hash([]byte(fmt.Sprintf("SIGNATURE:1:%d", time.Now().UnixNano())))
			key := crypto.NewKeyFromSeed(append(sh0[:], sh1[:]...))
			sig := key.Sign(sh0)
			s := common.Snapshot{
				Version:      common.SnapshotVersionCommonEncoding,
				NodeId:       crypto.Blake3Hash([]byte(fmt.Sprint(n))),
				References:   &common.RoundLink{Self: self, External: external},
				RoundNumber:  uint64(time.Now().Unix()/100000) + uint64(i*n),
				Timestamp:    uint64(time.Now().UnixNano()),
				Signature:    &crypto.CosiSignature{Signature: sig, Mask: uint64(time.Now().UnixNano())},
				Transactions: []crypto.Hash{tx},
			}
			topo := common.SnapshotWithTopologicalOrder{Snapshot: &s, TopologicalOrder: uint64(i * n)}
			s.Hash = s.PayloadHash()
			val := topo.VersionedMarshal()
			err := os.WriteFile(dir+"/SNAPSHOT-"+s.Hash.String(), val, 0644)
			if err != nil {
				panic(err)
			}
		}
	}
}
