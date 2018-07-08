#!/bin/sh

GOOS=js GOARCH=wasm go1.11beta1 build -o main.wasm ./main.go
