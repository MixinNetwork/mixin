#!/bin/sh

sed -i --  "s/BUILD_VERSION/`git rev-parse HEAD`/g" config/config.go || exit
go build
