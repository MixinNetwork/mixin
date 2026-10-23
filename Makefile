.PHONY: all

all:
	build_commit=$$(git rev-parse --short HEAD) && \
	CGO_ENABLED=0 go build -ldflags "-X github.com/MixinNetwork/mixin/config.buildCommit=$$build_commit" -o mixin
