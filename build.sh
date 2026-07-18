#!/bin/bash

echo "Building emba-api..."
go build -ldflags="-s -w" -o ./build/emba-api .