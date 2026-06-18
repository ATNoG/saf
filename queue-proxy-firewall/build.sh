#!/bin/bash

export KO_DOCKER_REPO='ghcr.io/atnog/serverless-workflow-firewall'

# Copy the most up to date schema
cp ../rules-reference/firewall-schema.yaml ./pkg/firewall

ko build ./cmd/queue --tags=latest -B
