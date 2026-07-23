default:
  just --list
dev: 
  go run ./cmd/main.go -d

clean: 
  rm -rf ./dev-output/**/*
