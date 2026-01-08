#!/bin/bash

kubectl patch configmap config-deployment -n knative-serving \
  --type merge \
  -p '{"data":{"queue-sidecar-image":"ghcr.io/atnog/serverless-workflow-firewall/queue:latest"}}'

kubectl patch configmap config-features -n knative-serving \
  --type merge \
  -p '{"data":{"queueproxy.mount-podinfo":"enabled"}}'
