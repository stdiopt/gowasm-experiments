#!/bin/sh
# Only works with go above 1.11beta1 current master (07-jun-2018)
GOOS=js GOARCH=wasm go.master build -o main.wasm ./main.go
