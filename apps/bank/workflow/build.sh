#!/bin/bash

# REGISTRY=10.43.142.183:5000/knative-workflow-apps-kit/bank-app
REGISTRY=ghcr.io/atnog/knative-workflow-apps-kit/bank-app

kubectl delete ksvc workflow -n bank-app
kn workflow quarkus build --image=workflow --jib
docker image tag workflow $REGISTRY/workflow
docker push $REGISTRY/workflow
kn workflow quarkus deploy  --path ./src/main/kubernetes
