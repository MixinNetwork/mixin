all:
	git checkout config/reader.go
	sed -i --  "s/BUILD_VERSION/`git rev-parse HEAD`/g" config/reader.go || exit
	CGO_ENABLED=0 go build -o mixin
