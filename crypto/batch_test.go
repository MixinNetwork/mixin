package crypto

import (
	"fmt"
	"testing"
)

func BenchmarkVerifyBatch(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8, 64, 256} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			msg := []byte("BenchmarkVerifyBatch")
			var pubs []*Key
			var sigs []*Signature
			for i := 0; i < n; i++ {
				seed := []byte(fmt.Sprintf("SEED%060d", i*128))
				priv := NewKeyFromSeed(seed)
				pub := priv.Public()
				sig := priv.Sign(msg)
				pubs = append(pubs, &pub)
				sigs = append(sigs, &sig)
			}
			for i := 0; i < b.N/n; i++ {
				if !BatchVerify(msg, pubs, sigs) {
					b.Fatal("batch verification")
				}
			}
		})
	}
}
