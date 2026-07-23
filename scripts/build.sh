#!/bin/sh

go build -ldflags="-s -w" -o library-syncer ./cmd/main.go
