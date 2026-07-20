default:
  just --list
dev: 
  go run ./cmd/main.go

clean: 
  rm -rf ./dev-output/
