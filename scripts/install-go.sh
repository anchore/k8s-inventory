#!/usr/bin/env bash

GO_VERSION="1.25.1"
ARCH=$(uname | tr '[:upper:]' '[:lower:]')
GO_ARTIFACT="go$GO_VERSION.$ARCH-amd64.tar.gz"
wget "https://dl.google.com/go/$GO_ARTIFACT"

tar -xvf "$GO_ARTIFACT"
mv go /usr/local

export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

go version
