#!/bin/bash

# REGISTRY=10.43.142.183:5000/knative-workflow-apps-kit/bank-app
REGISTRY=ghcr.io/atnog/knative-workflow-apps-kit/bank-app
paths=(entry-point login authorization verify-transaction transaction result)

for p in ${paths[@]}; do
    cd $p
    docker build -t="$REGISTRY/$p" .
    docker push "$REGISTRY/$p"
    cd ../
done
