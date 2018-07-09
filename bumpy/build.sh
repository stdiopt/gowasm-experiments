#!/bin/sh

GOOS=js GOARCH=wasm go.master build -o main.wasm ./main.go
