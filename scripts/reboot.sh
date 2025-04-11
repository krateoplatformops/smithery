#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

${SCRIPT_DIR}/kind-down.sh

${SCRIPT_DIR}/kind-up.sh

${SCRIPT_DIR}/build.sh


kubectl apply -f manifests/

