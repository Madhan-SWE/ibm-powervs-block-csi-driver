#!/bin/bash

set -euo pipefail

function clusterctl_install() {
  INSTALL_PATH=${1}
  CLUSTERCTL_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/eksctl ]]; then
    CLUSTERCTL_DOWNLOAD_URL="https://github.com/kubernetes-sigs/cluster-api/releases/download/${CLUSTERCTL_VERSION}/clusterctl-darwin-amd64"
    curl --silent --location "${CLUSTERCTL_DOWNLOAD_URL}" -o clusterctl
    chmod +x ./clusterctl
    mv clusterctl "${INSTALL_PATH}"
  fi
}

function clusterctl_create_cluster(){

    echo "------ $@ -----"
    CLUSTERCTL_BIN=${1}
    SSHKEY_NAME=${2}
    IBMPOWERVS_VIP=${3}
    VIP_EXTERNAL=${4}
    VIP_CIDR=${5}
    IMAGE_NAME=${6}
    SERVICE_INSTANCE_ID=${7}
    NETWORK_NAME=${8}
    CLUSTER_NAME=${9}
    KUBERNETES_VERSION=${10}
    TARGET_NAMESPACE=${11}
    CONTROL_PLANE_MACHINE_COUNT=${12}
    WORKER_MACHINE_COUNT=${13}
    CLUSTER_TEMPLATE_FILE=${14}
    
    OUTPUT=""" IBMPOWERVS_SSHKEY_NAME="${SSHKEY_NAME}" \
    IBMPOWERVS_VIP="${VIP}" \
    IBMPOWERVS_VIP_EXTERNAL="${VIP_EXTERNAL}" \
    IBMPOWERVS_VIP_CIDR="${VIP_CIDR}" \
    IBMPOWERVS_IMAGE_NAME="${IMAGE_NAME}" \
    IBMPOWERVS_SERVICE_INSTANCE_ID="${SERVICE_INSTANCE_ID}" \
    IBMPOWERVS_NETWORK_NAME="${NETWORK_NAME}" \
    ${CLUSTERCTL_BIN} generate cluster ${CLUSTER_NAME} --kubernetes-version ${KUBERNETES_VERSION} \
    --target-namespace ${TARGET_NAMESPACE} \
    --control-plane-machine-count=${CONTROL_PLANE_MACHINE_COUNT} \
    --worker-machine-count=${WORKER_MACHINE_COUNT} \
    --from ${CLUSTER_TEMPLATE_FILE}"""

    echo $OUTPUT


    OUTPUT=$(IBMPOWERVS_SSHKEY_NAME="${SSHKEY_NAME}" \
    IBMPOWERVS_VIP="${VIP}" \
    IBMPOWERVS_VIP_EXTERNAL="${VIP_EXTERNAL}" \
    IBMPOWERVS_VIP_CIDR="${VIP_CIDR}" \
    IBMPOWERVS_IMAGE_NAME="${IMAGE_NAME}" \
    IBMPOWERVS_SERVICE_INSTANCE_ID="${SERVICE_INSTANCE_ID}" \
    IBMPOWERVS_NETWORK_NAME="${NETWORK_NAME}" \
    ${CLUSTERCTL_BIN} generate cluster ${CLUSTER_NAME} --kubernetes-version ${KUBERNETES_VERSION} \
    --target-namespace ${TARGET_NAMESPACE} \
    --control-plane-machine-count=${CONTROL_PLANE_MACHINE_COUNT} \
    --worker-machine-count=${WORKER_MACHINE_COUNT} \
    --from ${CLUSTER_TEMPLATE_FILE} | kubectl apply -f -)

    echo "OUTPUT ==========: $OUTPUT"

    ## Process the output

}

function clusterctl_delete_cluster(){
    CLUSTER_NAME={1}
    OUTPUT=$(clusterctl delete cluster ${CLUSTER_NAME})
    # process the output
}
