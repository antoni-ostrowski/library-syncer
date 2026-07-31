#!/bin/sh

scripts/clean.sh &&
templ generate --watch --cmd="scripts/dev.sh"


