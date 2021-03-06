#!/bin/bash
CMDNAME=`basename $0`
if [ $# -ne 4 ]; then
  echo "Usage: $CMDNAME <NAMESPACE> <PUBRING-KEY-NAME> <PUBRING-KEY-FILE-PATH> <PLACEMENT-RULE-KEY-VALUE-PAIR>" 1>&2
  echo "E.g.:  ./acm-verification-key-setup \\
		integrity-shield-operator-system \\
                keyring-secret \\
	        /tmp/pubring.gpg \\
		environment:dev" \\
  exit 1
fi

if ! [ -x "$(command -v kubectl)" ]; then
    echo 'Error: kubectl is not installed.' >&2
    exit 1
fi

NAMESPACE=$1
PUBRING_KEY_NAME=$2
PUBRING_KEY_FILE_PATH=$3
PLACEMENT_KEY_VALUE=$4

if [ -z "$PLACEMENT_KEY_VALUE" ]; then
    echo "Please pass <PLACEMENT-RULE-KEY-VALUE-PAIR> as parameter e.g. 'environment:dev'"
    exit 1
else
    PLACEMENT_KEY=$(echo ${PLACEMENT_KEY_VALUE} | cut -d':' -f1)
    PLACEMENT_VALUE=$(echo ${PLACEMENT_KEY_VALUE} | cut -d':' -f2)
fi


if [ -z "$PLACEMENT_KEY" ]; then
    echo "Please pass <PLACEMENT-RULE-KEY-VALUE-PAIR> as parameter e.g. 'environment:dev'"
    exit 1
fi


if [ -z "$PLACEMENT_VALUE" ]; then
    echo "Please pass <PLACEMENT-RULE-KEY-VALUE-PAIR> as parameter e.g. 'environment:dev'"
    exit 1
fi


if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    BASE='base64 -w 0'
elif [[ "$OSTYPE" == "darwin"* ]]; then
    BASE='base64'
fi


cat <<EOF
apiVersion: v1
data:
  pubring.gpg: `cat ${PUBRING_KEY_FILE_PATH} | ${BASE}`
kind: Secret
metadata:
  annotations:
    apps.open-cluster-management.io/deployables: "true"
  name: ${PUBRING_KEY_NAME}
  namespace: ${NAMESPACE}
type: Opaque
---
apiVersion: apps.open-cluster-management.io/v1
kind: Channel
metadata:
  name: ${PUBRING_KEY_NAME}-deployments
  namespace: ${NAMESPACE}
spec:
  pathname: ${NAMESPACE}
  sourceNamespaces:
  - ${NAMESPACE}
  type: Namespace
---
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
  name: secret-placement
  namespace: ${NAMESPACE}
spec:
  clusterConditions:
  - status: "True"
    type: ManagedClusterConditionAvailable
  clusterSelector:
    matchExpressions:
    - key: ${PLACEMENT_KEY}
      operator: In
      values:
      - ${PLACEMENT_VALUE}
---
apiVersion: apps.open-cluster-management.io/v1
kind: Subscription
metadata:
  name: ${PUBRING_KEY_NAME}
  namespace: ${NAMESPACE}
spec:
  channel: ${NAMESPACE}/${PUBRING_KEY_NAME}-deployments
  placement:
    placementRef:
      kind: PlacementRule
      name: secret-placement
EOF
