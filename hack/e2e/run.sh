#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

set -euo pipefail

# directories
BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
TEST_DIR=${BASE_DIR}/csi-test-artifacts
BIN_DIR=${TEST_DIR}/bin

# sources
source "${BASE_DIR}"/util.sh
source "${BASE_DIR}"/prerequisites.sh
source "${BASE_DIR}"/clusterctl.sh

# prerequisites and versions
KIND_VERSION=${KIND_VERSION:-v0.12.0}
PVSADM_VERSION=${PVSADM_VERSION:-v0.1.4}
IBMCLOUDCLI_VERSION=${IBMCLOUDCLI_VERSION:-2.6.0}
KUBECTL_VERSION=${KUBECTL_VERSION:-v1.23.0}
CLUSTERCTL_VERSION=${CLUSTERCTL_VERSION:-v1.1.3}

# power vs network configuration
NETWORK_NAME=${NETWORK_NAME:-capi-test-madhan}
PORT_DESCRIPTION=${PORT_DESCRIPTION:-capi-port}
DNS_SERVERS=${DNS_SERVERS:-8.8.8.8 9.9.9.9}
SERVICE_INSTANCE_ID=${SERVICE_INSTANCE_ID:-97ff60d4-5b60-4a3d-bb28-34aedc603bf3}
CRN=${CRN:-crn:v1:bluemix:public:power-iaas:lon04:a/65b64c1f1c29460e8c2e4bbfbd893c2c:97ff60d4-5b60-4a3d-bb28-34aedc603bf3::}

# powervs csi driver configuration
POWERVS_CSI_DRIVER_VERSION=${POWERVS_CSI_DRIVER:-v0.1.0}

# ginkgo configuration
TEST_PATH=${TEST_PATH:-"./tests/e2e/..."}
ARTIFACTS=${ARTIFACTS:-"${TEST_DIR}/artifacts"}
GINKGO_FOCUS=${GINKGO_FOCUS:-"\[ebs-csi-e2e\]"}
GINKGO_SKIP=${GINKGO_SKIP:-"\[Disruptive\]"}
GINKGO_NODES=${GINKGO_NODES:-2}
TEST_EXTRA_FLAGS=${TEST_EXTRA_FLAGS:-}

# capi cluster deployment configuration
CLUSTER_NAME=${CLUSTER_NAME:-ibm-powervs-e2e-test-1}
IMAGE_NAME=${IMAGE_NAME:-capibm-powervs-centos-streams8-1-22-4}
KUBERNETES_VERSION=${KUBECTL_VERSION:-v1.22.4}
TARGET_NAMESPACE=${TARGET_NAMESPACE:-default}
CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-3}
WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-1}
CLUSTER_TEMPLATE_FILE=${CLUSTER_TEMPLATE_FILE:-./hack/e2e/cluster-template-powervs.yaml}
SSHKEY_NAME=${SSHKEY_NAME:-Madhan-ssh-key}
VIP_CIDR=${VIP_CIDR:-29}

# creating bin directory
mkdir -p "${BIN_DIR}"

# Installing prerequisites to BIN_DIR
# Installing kubectl
loudecho "Installing kubectl ${KUBECTL_VERSION} to ${BIN_DIR}"
# kubectl_install "${BIN_DIR}" "${KUBECTL_VERSION}"
KUBECTL_BIN="${BIN_DIR}/kubectl"

# Installing kind
loudecho "Installing kind ${KIND_VERSION} to ${BIN_DIR}"
# kind_install "${BIN_DIR}" "${KIND_VERSION}"
KIND_BIN="${BIN_DIR}/kind"

# Installing pvsadm
loudecho "Installing pvsadm ${PVSADM_VERSION} to ${BIN_DIR}"
# pvsadm_install "${BIN_DIR}" "${PVSADM_VERSION}"
PVSADM_BIN="${BIN_DIR}/pvsadm"

# Installing ibmcloud cli
loudecho "Installing ibmcloud cli ${IBMCLOUDCLI_VERSION} to ${BIN_DIR}"
# ibmcloudcli_install "${BIN_DIR}" "${IBMCLOUDCLI_VERSION}"
IBMCLOUD_CLI_BIN="${BIN_DIR}/Bluemix_CLI/bin/ibmcloud"
IBMCLOUD_CLI_BIN="ibmcloud"

# Installing clusterctl
loudecho "Installing clusterctl ${CLUSTERCTL_VERSION} to ${BIN_DIR}"
# clusterctl_install "${BIN_DIR}" "${CLUSTERCTL_VERSION}"
CLUSTERCTL_BIN="${BIN_DIR}/clusterctl"

# creating kind cluster
# ${KIND_BIN} create cluster

set -x

# choosing target for ibmcloud cli
${IBMCLOUD_CLI_BIN} pi service-target $CRN

# create public network
loudecho "Creating Public network ${NETWORK_NAME}"
NETWORK=$(${IBMCLOUD_CLI_BIN} pi network-create-public ${NETWORK_NAME} --dns-servers "${DNS_SERVERS}")
echo "NETWORK: $NETWORK"


# create powervs network port
loudecho "Creating powervs network port ${PORT_DESCRIPTION}"
# OUTPUT=$(${PVSADM_BIN} create port --description "${PORT_DESCRIPTION}" --network "${NETWORK_NAME}" --instance-id "${SERVICE_INSTANCE_ID}")
# echo "-----------------  OUTPUT Create port ------------------------ ${OUTPUT}"
# VIP_EXTERNAL=$(echo $OUTPUT | cut -d '|' -f12  | tr -d ' ')
# VIP=$(echo $OUTPUT | cut -d '|' -f14  | tr -d ' ')

PORTS=$(${PVSADM_BIN} get ports --network "${NETWORK_NAME}" --instance-id "${SERVICE_INSTANCE_ID}")
echo "PORTS - - - - : $PORTS"
VIP_EXTERNAL=$(echo $PORTS | cut -d '|' -f10  | tr -d ' ')
VIP=$(echo $PORTS | cut -d '|' -f11  | tr -d ' ')
PORT_ID=$(echo $PORTS | cut -d '|' -f13  | tr -d ' ')
echo "VIP: $VIP VIP_EXTERNAL: $VIP_EXTERNAL PORT_ID: $PORT_ID"

# VIP_EXTERNAL="130.198.103.115"
# VIP="192.168.155.115"

# echo "VIP_EXTERNAL=$(echo $OUTPUT | cut -d '|' -f12  | tr -d ' ')"
# echo "VIP=$(echo $OUTPUT | cut -d '|' -f14  | tr -d ' ')"

set +x

loudecho "Creating cluster ${CLUSTER_NAME}"
echo "SSHKEY_NAME $SSHKEY_NAME-- /n" \
    "VIP $VIP-- /n" \
    "VIP_EXTERNAL $VIP_EXTERNAL-- /n" \
    "VIP_CIDR $VIP_CIDR-- /n" \
    "IMAGE_NAME $IMAGE_NAME-- /n" \
    "SERVICE_INSTANCE_ID $SERVICE_INSTANCE_ID-- /n" \
    "NETWORK_NAME $NETWORK_NAME--  /n" \
    "CLUSTER_NAME $CLUSTER_NAME-- /n" \
    "KUBERNETES_VERSION $KUBERNETES_VERSION-- /n" \
    "TARGET_NAMESPACE $TARGET_NAMESPACE /n" \
    "CONTROL_PLANE_MACHINE_COUNT $CONTROL_PLANE_MACHINE_COUNT-- /n" \
    "WORKER_MACHINE_COUNT $WORKER_MACHINE_COUNT-- /n" \
    "CLUSTER_TEMPLATE_FILE $CLUSTER_TEMPLATE_FILE-- /n"

# creating capi cluster
clusterctl_create_cluster \
    "$CLUSTERCTL_BIN" \
    "$SSHKEY_NAME" \
    "$VIP" \
    "$VIP_EXTERNAL" \
    "$VIP_CIDR" \
    "$IMAGE_NAME" \
    "$SERVICE_INSTANCE_ID" \
    "$NETWORK_NAME" \
    "$CLUSTER_NAME" \
    "$KUBERNETES_VERSION" \
    "$TARGET_NAMESPACE" \
    "$CONTROL_PLANE_MACHINE_COUNT" \
    "$WORKER_MACHINE_COUNT" \
    "$CLUSTER_TEMPLATE_FILE"


if [[ $? -ne 0 ]]; then
    exit 1
fi

set +x

# get and export the kubeconfig
clusterctl get kubeconfig $CLUSTER_NAME > ~/.kube/$CLUSTER_NAME
export KUBECONFIG=~/.kube/$CLUSTER_NAME

#get nodes
kubectl get nodes

# ginkgo installation
loudecho "Installing ginkgo to ${BIN_DIR}"
GINKGO_BIN=${BIN_DIR}/ginkgo
if [[ ! -e ${GINKGO_BIN} ]]; then
  pushd /tmp
  GOPATH=${TEST_DIR} GOBIN=${BIN_DIR} GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo@v1.12.0
  popd
fi

# csi driver deployment
loudecho "Deploying driver"
kubectl apply --kubeconfig "${KUBECONFIG}" -k "https://github.com/kubernetes-sigs/ibm-powervs-block-csi-driver/deploy/kubernetes/overlays/stable/?ref=${POWERVS_CSI_DRIVER_VERSION}"
loudecho "Driver deployment completed"

loudecho "Running tests"
eval "EXPANDED_TEST_EXTRA_FLAGS=$TEST_EXTRA_FLAGS"
set -x
set +e
${GINKGO_BIN} -p -nodes="${GINKGO_NODES}" -v --focus="${GINKGO_FOCUS}" --skip="${GINKGO_SKIP}" "${TEST_PATH}" -- -kubeconfig="${KUBECONFIG}" -report-dir="${ARTIFACTS}"
TEST_PASSED=$?
set -e
set +x
loudecho "TEST_PASSED: ${TEST_PASSED}"

PODS=$(kubectl get pod -n kube-system -o json --kubeconfig "${KUBECONFIG}" | jq -r .items[].metadata.name)

while IFS= read -r POD; do
  loudecho "Printing pod ${POD} ${CONTAINER_NAME} container logs"
  set +e
  kubectl logs "${POD}" -n kube-system "${CONTAINER_NAME}" \
    --kubeconfig "${KUBECONFIG}"
  set -e
done <<< "${PODS}"

# deleting capi cluster
$CLUSTERCTL_BIN} delete cluster ${CLUSTER_NAME}

# delete network port
${PVSADM_BIN} delete port --port-id "${PORT_ID}" --network "${NETWORK_NAME}" --instance-id "${SERVICE_INSTANCE_ID}")

# delete network
${IBMCLOUD_CLI_BIN} pi network delete ${NETWORK_ID}