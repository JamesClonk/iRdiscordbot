#!/bin/bash

# fail on error
set -e

# =============================================================================================
if [[ "$(basename $PWD)" == "scripts" ]]; then
    cd ..
fi
echo $PWD

# =============================================================================================
source .env
source ~/.config/irdiscordbot.conf || true

# =============================================================================================
echo "developing irvisualizer ..."
killall gin-bin || true
killall irdiscordbot || true
rm -f gin-bin || true
#gin --all run main.go

rm -f irdiscordbot || true
GOARCH=amd64 GOOS=linux go build -i -o irdiscordbot
./irdiscordbot
