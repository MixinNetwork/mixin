#!/bin/sh

sudo systemctl stop mixin-archive || exit 1

tar cf - -C /mnt/archive/mixin snapshots | s3cmd put - s3://mixin/snapshots/kernel.tar.new || exit 1

s3cmd mv s3://mixin/snapshots/kernel.tar.new s3://mixin/snapshots/kernel.tar || exit 1

s3cmd setacl s3://mixin/snapshots/kernel.tar --acl-public || exit 1

s3cmd la --recursive || exit 1

sudo systemctl restart mixin-archive
