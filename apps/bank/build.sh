#!/bin/bash

REGISTRY=ghcr.io/atnog/serverless-workflow-firewall/bank-app
paths=(entry-point login authorization verify-transaction transaction result)

for p in ${paths[@]}; do
    cd $p
    docker build -t="$REGISTRY/$p:latest" .
    docker push "$REGISTRY/$p:latest"
    cd ../
done
