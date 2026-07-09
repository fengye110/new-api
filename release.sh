#!/bin/bash

VERSION=$(git describe --tags --always --dirty)

cd web

bun install --frozen-lockfile
cd default
DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION="$VERSION" bun run build

cd ../classic
VITE_REACT_APP_VERSION="$VERSION" bun run build

cd ../..

mkdir -p dist

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -trimpath \
  -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${VERSION}'" \
  -o dist/new-api-linux-amd64 .