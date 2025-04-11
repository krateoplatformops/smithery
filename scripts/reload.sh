#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

kubectl delete -f manifests/deploy.smithery.yaml

${SCRIPT_DIR}/build.sh

kubectl apply -f manifests/deploy.smithery.yaml

