#!/bin/sh

# execute this first
# go install github.com/swaggo/swag/cmd/swag@latest

swag init --dir backend/ -o docs --parseDependency --parseInternal --exclude fast-note-sync-service-source
