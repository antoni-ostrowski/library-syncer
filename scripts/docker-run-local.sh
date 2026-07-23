#!/bin/sh


docker run  \
  -v ./dev/docker-data/:/app/data \
  -v ~/.ssh/id_ed25519:/root/.ssh/id_ed25519:ro \
  -v ~/.ssh/known_hosts:/root/.ssh/known_hosts:ro \
  --env-file ./scripts/.docker-env \
  antost360/library-syncer:latest

