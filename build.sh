#!/bin/sh

git checkout config/config.go
sed -i --  "s/BUILD_VERSION/`git rev-parse HEAD`/g" config/config.go || exit
CGO_ENABLED=0 go build
