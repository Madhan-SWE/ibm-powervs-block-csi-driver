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
PVSADM_VERSION=${PVSADM_VERSION:-v0.1.4 }
IBMCLOUDCLI_VERSION=${IBMCLOUDCLI_VERSION:-2.6.0}
KUBECTL_VERSION=${KUBECTL_VERSION:-v1.23.0}
CLUSTERCTL_VERSION=${CLUSTERCTL_VERSION:-v1.1.3}

# power vs network configuration
NETWORK_NAME=${NETWORK_NAME:-capi-test}
PORT_DESCRIPTION=${PORT_DESCRIPTION:-capi-port}
DNS_SERVERS=${DNS_SERVERS:-8.8.8.8 9.9.9.9}
SERVICE_INSTANCE_ID=${SERVICE_INSTANCE_ID:-7845d372-d4e1-46b8-91fc-41051c984601}

# capi cluster deployment configuration
CLUSTER_NAME=${CLUSTER_NAME:-ibm-powervs-1}
IMAGE_NAME=${IMAGE_NAME:-capibm-powervs-centos-8-1-22-4}
NETWORK_NAME=${NETWORK_NAME:-capi-test-3}
KUBERNETES_VERSION=${KUBECTL_VERSION:-v1.22.4}
TARGET_NAMESPACE=${TARGET_NAMESPACE:-default}
CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-3}
WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-1}
CLUSTER_TEMPLATE_FILE=${CLUSTER_TEMPLATE_FILE:-./cluster-template-powervs.yaml}
SSHKEY_NAME=${SSHKEY_NAME:-my-pub-key}
VIP_CIDR=${VIP_CIDR:-29}

# Installing prerequisites to BIN_DIR
# Installing kubectl
loudecho "Installing kubectl ${KUBECTL_VERSION} to ${BIN_DIR}"
kubectl_install "${BIN_DIR}" "${KUBECTL_VERSION}"
KUBECTL_BIN="${BIN_DIR}/kubectl"

# Installing kind
loudecho "Installing kind ${KIND_VERSION} to ${BIN_DIR}"
kind_install "${BIN_DIR}" "${KIND_VERSION}"
KIND_BIN="${BIN_DIR}/kind"

# Installing pvsadm
loudecho "Installing pvsadm ${PVSADM_VERSION} to ${BIN_DIR}"
pvsadm_install "${BIN_DIR}" "${PVSADM_VERSION}"
PVSADM_BIN="${BIN_DIR}/pvsadm"

# Installing ibmcloud cli
loudecho "Installing ibmcloud cli ${IBMCLOUDCLI_VERSION} to ${BIN_DIR}"
ibmcloudcli_install "${BIN_DIR}" "${IBMCLOUDCLI_VERSION}"
IBMCLOUD_CLI_BIN="${BIN_DIR}/bin/Bluemix_CLI/bin/ibmcloud"

# Installing clusterctl
loudecho "Installing clusterctl ${CLUSTERCTL_VERSION} to ${BIN_DIR}"
clusterctl_install "${BIN_DIR}" "${CLUSTERCTL_VERSION}"
CLUSTERCTL_BIN="${BIN_DIR}/clusterctl"

# creating kind cluster
${KIND_BIN} create cluster

# create public network
${IBMCLOUD_CLI_BIN} pi network-create-public ${NETWORK_NAME} --dns-servers ${DNS_SERVERS}

# create powervs network port
OUTPUT=$(${PVSADM_BIN} --description "${PORT_DESCRIPTION}" --network "${NETWORK_NAME}" --instance-id "${SERVICE_INSTANCE_ID}")
echo "-----------------  OUTPUT Create port ------------------------ ${OUTPUT}"
VIP_EXTERNAL=$(echo $OUTPUT | grep "${PORT_DESCRIPTION}" | cut -d '|' -f3  | tr -d ' ')
VIP=$(echo $OUTPUT | grep "${PORT_DESCRIPTION}" | cut -d '|' -f4  | tr -d ' ')

echo "SSHKEY_NAME $SSHKEY_NAME \n" \
    "VIP $VIP \n" \
    "VIP_EXTERNAL $VIP_EXTERNAL \n" \
    "VIP_CIDR $VIP_CIDR\n" \
    "IMAGE_NAME $IMAGE_NAME \n" \
    "SERVICE_INSTANCE_ID $SERVICE_INSTANCE_ID \n" \
    "NETWORK_NAME $NETWORK_NAME  \n" \
    "CLUSTER_NAME $CLUSTER_NAME \n" \
    "KUBERNETES_VERSION $KUBERNETES_VERSION \n" \
    "TARGET_NAMESPACE $TARGET_NAMESPACE \n" \
    "CONTROL_PLANE_MACHINE_COUNT $CONTROL_PLANE_MACHINE_COUNT \n" \
    "WORKER_MACHINE_COUNT $WORKER_MACHINE_COUNT \n" \
    "CLUSTER_TEMPLATE_FILE $CLUSTER_TEMPLATE_FILE \n"

# creating capi cluster
clusterctl_create_cluster \
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



# # deleting capi cluster
# $CLUSTERCTL_BIN} delete cluster ${CLUSTER_NAME}
# if [[ $? -ne 0 ]]; then
#     exit 1
# fi