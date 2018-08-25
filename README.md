# go 1.11 webassembly experiments

* [bouncy](https://stdiopt.github.io/gowasm-experiments/bouncy)
* [rainbow-mouse](https://stdiopt.github.io/gowasm-experiments/rainbow-mouse)
* [repulsion](https://stdiopt.github.io/gowasm-experiments/repulsion)
* [bumpy](https://stdiopt.github.io/gowasm-experiments/bumpy)
* [splashy](https://stdiopt.github.io/gowasm-experiments/splashy)

## getting go1.11

install latest go 1.11 https://golang.org/dl/

## Building and running

```sh
$ cd {proj} # sub folder (i.e. bouncy, rainbow-mouse)
$ go get -v # ignore the js warning
$ ./build.sh
$ caddy
```

Serve with caddy or anything else that is able to set the mimetype
'application/wasm' for .wasm files
