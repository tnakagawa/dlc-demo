#!/bin/bash

# exit immediately if any error occurred
set -e

echo "===== build start ====="

GO=go

cd $(dirname $0)

export GOPATH=$(pwd)

OUTDIR=demo

echo "GOPATH=$GOPATH"

if [ ! -e "$OUTDIR" ]; then
    mkdir "$OUTDIR"
    echo "mkdir $OUTDIR"
fi

LIBS=("github.com/btcsuite/btcd"
      "github.com/btcsuite/btcutil")

for lib in ${LIBS[@]}; do
    echo "$GO get $lib"
    $GO get $lib
done

target="demo"
printf "==== %4s build start ====\n" "$target"
cd "$GOPATH/src/$target"
$GO build -o "../../$OUTDIR/$target" -v
printf "==== %4s build end ====\n" "$target"

echo "===== buid end ===="
