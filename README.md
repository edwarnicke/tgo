tgo (prononuced 'to go') is a transparent drop in **wrapper** around go that allows for easy fast docker containers to be built in a go repo.

Try it now:

```dockerfile
FROM ${baseimg}
RUN go get github.com/edwarnicke/tgo/cmd/tgo
RUN tgo build -o . ./...
```

```bash
go get github.com/edwarnicke/tgo/cmd/tgo
tgo docker build .
```

# The Problem

Both go and docker have marvelous developer usability.  When used together, there some rough spots.

1. Redownload of go dependency source on each docker build slows down docker builds
1. Local go.mod replace directives (replace github.com/foo => ../foo) breaks docker builds entirely

Download of dependency source can be somewhat improved by docker layer caching using techniques like

```dockerfile
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
```

But it introduces additional complexity into the Dockerfile primarily to hack around the impedance mismatch.

Local replace directives are part of life in working in highly modular multi-repo projects.  The inability to do a docker build
involving more than one modified local repos is quite limiting.

## The Solution
tgo localizes the docker GOPATH and source code from local 'replace' directives in the go.mod into a ./.tgo subdirectory.
When you do a ```docker build .``` all of that context is sent off to the docker server and is available.

tgo also *uses* that context when building.  In a Dockerfile that has a .tgo directory available using

```bash
tgo build -o . ./...
```
or
```bash
tgo test ./...
```

or

```go
tgo ${anything you'd pass to go normally}
```

Will utilize the GOPATH, and copies of local dependency source from replace directives transparently.
The ```go``` in your path is used under the covers.

## Use with Docker
Replace go with tgo in the Dockerfile:

```dockerfile
FROM ${baseimg}
RUN go get github.com/edwarnicke/tgo/cmd/tgo
RUN tgo build -o . ./...
```

```docker build .``` will work normally.

If on the local host tgo is used instead of go:

```tgo build ./...```

as part of development source code for dependencies and local replace directives will be used in the docker build.

### Single command-line use with Docker

Any non-go command can be run using tgo. tgo will first warm the .tgo cache and then run that command:

```bash
tgo docker build .
```

with effectively run:
```go
tgo build ./...
```
and then
```go
docker build .
```

## How it works under the covers

tgo is very simple.  Running 

```go
tgo ${anything you would pass to go}
```

will:

Create (if it doesn't already exist) directories .tgo and .tgo/${GOPATH} in the current directory.

Example:

```GOPATH=/home/bob/go```  =>  ```/home/bob/git/foo/.tgo/root/home/bob/go```

Create a symlink from 

```/home/bob/git/foo/.tgo/root/home/bob/git/foo/``` -> ```/home/bob/git/foo/```

copy any source dependencies on the local file system that are *not* in GOPATH or GOROOT into .tgo/root  

Example:

Source code is in:

```/home/bob/git/foo``` 

and the 

```/home/bob/git/foo/go.mod``` 

has a 

```replace github.com/bob/bar => ../bar``` 

replace directive, tgo will copy the contents of 

```/home/bob/git/bar``` 

to 

```/home/bob/git/foo/.tgo/root/home/bob/git/bar```

tgo notes the pkgdir (````/home/bob/git/foo````) and gopath (```/home/bob/go```) in 

```.tgo/env```

tgo runs the requested go command with 

```GOPATH=${PWD}/.tgo/root/home/bob/go```

```PWD=${PWD}/.tgo/root/home/bob/git/foo/```


