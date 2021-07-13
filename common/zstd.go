package common

import "github.com/klauspost/compress/zstd"

func NewZstdDecoder() *zstd.Decoder {
	opts := []zstd.DOption{
		zstd.WithDecoderDicts(ZstdEmbed),
		zstd.WithDecoderConcurrency(2),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxMemory(1024 * 1024 * 16),
	}
	dec, err := zstd.NewReader(nil, opts...)
	if err != nil {
		panic(err)
	}
	return dec
}

func NewZstdEncoder() *zstd.Encoder {
	opts := []zstd.EOption{
		zstd.WithEncoderDict(ZstdEmbed),
		zstd.WithEncoderLevel(3),
		zstd.WithWindowSize(8192),
	}
	enc, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		panic(err)
	}
	return enc
}
