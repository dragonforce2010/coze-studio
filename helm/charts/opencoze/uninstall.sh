#!/bin/bash

set -a; source .env; set +a;

NAMESPACE=${NAMESPACE:-"opencoze"}

helm uninstall opencoze --namespace ${NAMESPACE}
kubectl delete namespace ${NAMESPACE}