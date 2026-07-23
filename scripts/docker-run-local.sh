#!/bin/sh

docker run \
  -v ./data/secrets/:/app/data/secrets:ro \
  -v ~/.ssh/id_ed25519:/root/.ssh/id_ed25519:ro \
  -v ~/.ssh/known_hosts:/root/.ssh/known_hosts:ro \
  -e SSH_KEY=/root/.ssh/id_ed25519 \
  -e RSYNC_HOST=linux \
  -e RSYNC_USER=antost \
  -e RSYNC_SRC=./songs \
  -e RSYNC_DEST=/home/antost/mac-test \
  antost360/library-syncer:latest
