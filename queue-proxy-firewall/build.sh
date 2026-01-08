#!/bin/bash

export KO_DOCKER_REPO='ghcr.io/atnog/serverless-workflow-firewall'

ko build ./cmd/queue --tags=latest -B
