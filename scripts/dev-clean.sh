#!/bin/sh

scripts/clean.sh &&
templ generate --watch --proxy="http://localhost:8080" --cmd="scripts/dev.sh"


