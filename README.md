tgo (prononuced 'to go') is a transparent drop in **wrapper** around go that allows for easy fast docker containers to be built in a go repo.

# The Problem

Both go and docker have marvelous developer usability.  When used together, there are a few rough spots.

1. Redownload of go dependency source on each docker build slows down docker builds
1. Lack of go binary object cache slows down docker builds
1. Local go.mod replace directives (replace github.com/foo => ../foo) breaks docker builds entirely

Download of dependency source can be somewhat improved by docker layer caching using techniques like

```dockerfile
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
```

But it introduces additional complexity into the Dockerfile primarily to hack around the impedence mismatch.

Before tgo there are no strategies to deal with the go binary cache.  There are purests who would maintain that you **want** to 
regenerate that cache with each new docker build.  For production, this is unquestionably true.  For development, we can rely on the
extreme reproducibility of the go build process.

Local replace directives are part of life in working in highly modular multi-repo projects.  The inability to do a docker build
involving more than one modified local repos is quite limiting.

## The Solution
tgo localizes the docker GOPATH, GOCACHE, and source code from local 'replace' directives in the go.mod into a ./.tgo subdirectory.
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

Will utilize the GOPATH, GOCACHE, and copies of local dependency source from replace directives transparently.
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

as part of development, GOPATH and GOCACHE will be warm.  Additionally, local dependency source
for local replace directives will be used in the docker build.

Additionally, any non-go command can be run using tgo. tgo will first warm the .tgo cache and then run that command:

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

Create (if it doesn't already exist) a .tgo directory in the current directory

Create GOPATH and GOCACHE directories under .tgo as if they were relative to the hosts root directory

So if 

```GOPATH=/home/bob/go``` and ```GOCACHE=/home/bob/go-build``` 

then tgo will create directories

```/home/bob/git/foo/.tgo/root/home/bob/go``` and ```/home/bob/git/foo/.tgo/root/home/bob/go-build```

Create a symlink from 

```/home/bob/git/foo/.tgo/root/home/bob/git/foo/``` -> ```/home/bob/git/foo/```

Hardlink (or failing that copy) any source dependencies on the local file system that are *not* in GOPATH or GOROOT.
This will scoop up any local source dependencies from replace directives.  

Example:

So if the source code is in 

```/home/bob/git/foo``` 

and the 

```/home/bob/git/foo/go.mod``` 

has a 

```replace github.com/bob/bar => ../bar``` 

replace directive, tgo will hardlink (or failing that copy) the contents of 

```/home/bob/git/bar``` 

to 

```/home/bob/git/foo/.tgo/root/home/bob/git/bar```

tgo notes the pkgdir (````/home/bob/git/foo````) gocache (```/home/bob/go-build```) and gopath (```/home/bob/go```) in 

```.tgo/config```

tgo then runs the requested go command with 

```GOPATH=${PWD}/.tgo/root/home/bob/go```

```GOCACHE=${PWD}/.tgo/root/home/bob/go-build```

```PWD=${PWD}/.tgo/root/home/bob/git/foo/```


