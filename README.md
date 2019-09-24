# go webassembly experiments

Requires go1.13

- [bouncy](https://stdiopt.github.io/gowasm-experiments/bouncy)
- [rainbow-mouse](https://stdiopt.github.io/gowasm-experiments/rainbow-mouse)
- [repulsion](https://stdiopt.github.io/gowasm-experiments/repulsion)
- [bumpy](https://stdiopt.github.io/gowasm-experiments/bumpy)
- [splashy](https://stdiopt.github.io/gowasm-experiments/splashy)
- [arty](https://stdiopt.github.io/gowasm-experiments/arty/client)
- [hexy](https://stdiopt.github.io/gowasm-experiments/hexy)
- [bittune](https://stdiopt.github.io/gowasm-experiments/bittune)

## Building and running

```sh
$ cd {proj} # sub folder (i.e. bouncy, rainbow-mouse)
$ go get -v # ignore the js warning
$ ./build.sh
$ go run ../serve.go
```

Serve with caddy or anything else that is able to set the mimetype
'application/wasm' for .wasm files
