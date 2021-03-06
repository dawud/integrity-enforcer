#!/bin/bash
#
# Copyright 2020 IBM Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

CMDNAME=`basename $0`
if [ $# -ne 2 ]; then
  echo "Usage: $CMDNAME <signer> <tmp-dir>" 1>&2
  exit 1
fi

if [ ! -e $2 ]; then
  echo "$2 does not exist"
  exit 1
fi

if [ -z "$ISHIELD_REPO_ROOT" ]; then
    echo "ISHIELD_REPO_ROOT is empty. Please set env."
    exit 1
fi

SCRIPT_DIR=$(cd $(dirname $0); pwd)
SIGNER=$1
TMP_DIR="$2"/IV_TMP

KEY_EXIST=$(gpg --list-keys | grep ${SIGNER})

if  [ -z "${KEY_EXIST}" ]; then
    GEN_KEY=true
else
    GEN_KEY=false
fi

if [ ! -d ${TMP_DIR} ]; then
   echo "Creating tmpdir $TMP_DIR"
   mkdir ${TMP_DIR}
fi

INPUT_FILE=${TMP_DIR}/input.yaml
PUB_RING_KEY=${TMP_DIR}/pubring.gpg

define(){ IFS='\n' read -r -d '' ${1} || true; }

echo ""
define INPUT_YAML <<-EOF
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: policy-cert-ocp4
  annotations:
    policy.open-cluster-management.io/standards: NIST SP 800-53
    policy.open-cluster-management.io/categories: SC System and Communications Protection
    policy.open-cluster-management.io/controls: SC-12 Cryptographic Key Establishment and Management
spec:
  remediationAction: inform
  disabled: false
---
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  name: binding-policy-cert-ocp4
placementRef:
  name: placement-policy-cert-ocp4
  kind: PlacementRule
  apiGroup: apps.open-cluster-management.io
subjects:
- name: policy-cert-ocp4
  kind: Policy
  apiGroup: policy.open-cluster-management.io
---
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
  name: placement-policy-cert-ocp4
spec:
  clusterConditions:
  - status: "True"
    type: ManagedClusterConditionAvailable
  clusterSelector:
    matchExpressions:
      - {key: vendor, operator: In, values: ["OpenShift"]}
EOF

echo "$INPUT_YAML" > ${INPUT_FILE}

if [ -f ${INPUT_FILE} ]; then
   # Generating or exporting gpg key
   echo -----------------------------
   if [ "${GEN_KEY}" = true ]; then
     echo [1/4] Generating gpg key
   else
     echo [1/4] Exporting gpg key
   fi

   $SCRIPT_DIR/gen-gpg-key.sh "${SIGNER}" "${PUB_RING_KEY}" "${TMP_DIR}" "${GEN_KEY}"
   if [ ! -f ${PUB_RING_KEY} ]; then
        echo Failed to generate gpg key.
        exit 1
   fi
   echo done.
   echo -----------------------------
   echo ""

   # Generating signature annotation
   echo -----------------------------
   echo [2/4] Generating signature annotation
   ${ISHIELD_REPO_ROOT}/scripts/gpg-annotation-sign.sh ${SIGNER} ${INPUT_FILE}
   echo done.
   echo -----------------------------
   echo ""

   # Verifying signature annotation
   echo -----------------------------
   echo [3/4] Verifying signature annotation.
   ${ISHIELD_REPO_ROOT}/scripts/gpg-annotation-verify.sh ${INPUT_FILE} ${PUB_RING_KEY}
   echo done.
   echo -----------------------------
   echo ""
fi


if [ -d "${TMP_DIR}" ]; then
   # Removing  temp files.
   echo -----------------------------
   echo [4/4] Removing  temp files
   echo "${TMP_DIR} exist, removing it."
   rm -rf ${TMP_DIR}
   echo done.
   echo -----------------------------
   echo ""
fi

