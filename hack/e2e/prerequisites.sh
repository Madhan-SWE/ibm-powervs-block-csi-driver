# Install kubectl
function kubectl_install() {
  INSTALL_PATH=${1}
  KUBECTL_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/kubectl ]]; then
    KUBECTL_DOWNLOAD_URL="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
    curl --silent --location "${KUBECTL_DOWNLOAD_URL}" -o kubectl
    chmod +x ./kubectl
    mv kubectl "${INSTALL_PATH}"
  fi
}

# Install kind
function kind_install() {
  INSTALL_PATH=${1}
  KIND_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/kind ]]; then
    KIND_DOWNLOAD_URL="https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-$(uname)-amd64"
    curl --silent --location "${KIND_DOWNLOAD_URL}" -o kind
    chmod +x ./kind
    mv kind "${INSTALL_PATH}"
  fi
}

# Install pvsadm
function pvsadm_install() {
  INSTALL_PATH=${1}
  PVSADM_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/pvsadm ]]; then
    PVSADM_DOWNLOAD_URL="https://github.com/ppc64le-cloud/pvsadm/releases/download/${PVSADM_VERSION}/pvsadm-linux-amd64"
    curl --silent --location "${PVSADM_DOWNLOAD_URL}" -o pvsadm
    chmod +x ./pvsadm
    mv pvsadm "${INSTALL_PATH}"
  fi
}

# Install ibmcloud cli
function ibmcloudcli_install() {
  INSTALL_PATH=${1}
  IBMCLOUDCLI_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/eksctl ]]; then
    IBMCLOUDCLI_DOWNLOAD_URL="https://download.clis.cloud.ibm.com/ibm-cloud-cli/${IBMCLOUDCLI_VERSION}/IBM_Cloud_CLI_${IBMCLOUDCLI_VERSION}_amd64.tar.gz"
    curl --silent --location "${IBMCLOUDCLI_DOWNLOAD_URL}"| tar xz -C "${INSTALL_PATH}"
  fi
}