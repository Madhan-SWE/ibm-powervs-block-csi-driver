#!/bin/bash

set -euo pipefail

function clusterctl_install() {
  INSTALL_PATH=${1}
  CLUSTERCTL_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/eksctl ]]; then
    CLUSTERCTL_DOWNLOAD_URL="https://github.com/kubernetes-sigs/cluster-api/releases/download/${CLUSTERCTL_VERSION}/clusterctl-linux-amd64"
    curl --silent --location "${CLUSTERCTL_DOWNLOAD_URL}" -o clusterctl
    chmod +x ./clusterctl
    mv clusterctl "${INSTALL_PATH}"
  fi
}

function clusterctl_create_cluster(){
    SSHKEY_NAME={1}
    IBMPOWERVS_VIP={2}
    VIP_EXTERNAL={3}
    VIP_CIDR={4}
    IMAGE_NAME={5}
    SERVICE_INSTANCE_ID={6}
    NETWORK_NAME={7}
    KUBERNETES_VERSION={8}
    TARGET_NAMESPACE={9}
    CONTROL_PLANE_MACHINE_COUNT={10}
    WORKER_MACHINE_COUNT={11}
    CLUSTER_TEMPLATE_FILE={12}
    CLUSTER_NAME={13}

    OUTPUT=$(IBMPOWERVS_SSHKEY_NAME="${SSHKEY_NAME}" \
    IBMPOWERVS_VIP="${VIP}" \
    IBMPOWERVS_VIP_EXTERNAL="${VIP_EXTERNAL}" \
    IBMPOWERVS_VIP_CIDR="${VIP_CIDR}" \
    IBMPOWERVS_IMAGE_NAME="${IMAGE_NAME}" \
    IBMPOWERVS_SERVICE_INSTANCE_ID="${SERVICE_INSTANCE_ID}" \
    IBMPOWERVS_NETWORK_NAME="${NETWORK_NAME}" \
    clusterctl generate cluster ${CLUSTER_NAME} --kubernetes-version ${KUBERNETES_VERSIOn} \
    --target-namespace ${TARGET_NAMESPACE} \
    --control-plane-machine-count=${CONTROL_PLANE_MACHINE_COUNT} \
    --worker-machine-count=${WORKER_MACHINE_COUNT} \
    --from ${CLUSTER_TEMPLATE_FILE} | kubectl apply -f )

    ## Process the output

}

function clusterctl_delete_cluster(){
    CLUSTER_NAME={1}
    OUTPUT=$(clusterctl delete cluster ${CLUSTER_NAME})
    # process the output
}
