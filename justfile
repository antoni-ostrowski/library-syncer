default:
  just --list
dev: 
  RSYNC_SRC=./dev-output/ RSYNC_USER=antost RSYNC_HOSTNAME=linux RSYNC_DEST=/home/antost/mac-test SSH_KEY=~/.ssh/id_ed25519 go run ./cmd/main.go -d

clean: 
  rm -rf ./dev-output/**/*
