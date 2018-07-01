# go 1.11 webassembly experiments

## getting go1.11 beta

Get one of those
https://golang.org/dl/#go1.11beta1

and in my case I just unpacked in `/usr/lib/go`

## Building and running

```sh
$ cd {proj} # sub folder (i.e. bouncy, rainbow-mouse)
$ ./build.sh
$ caddy
```

Serve with caddy or anything else that is able to set the mimetype
'application/wasm' for .wasm files
