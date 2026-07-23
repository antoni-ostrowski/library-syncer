#!/bin/sh

export $(cat .env.local | xargs) && go run ./cmd/main.go -d
