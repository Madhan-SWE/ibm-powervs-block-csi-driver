function ibmcloudcli_install() {
  INSTALL_PATH=${1}
  IBMCLOUDCLI_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/eksctl ]]; then
    IBMCLOUDCLI_DOWNLOAD_URL="https://download.clis.cloud.ibm.com/ibm-cloud-cli/${IBMCLOUDCLI_VERSION}/IBM_Cloud_CLI_${IBMCLOUDCLI_VERSION}_amd64.tar.gz"
    curl --silent --location "${IBMCLOUDCLI_DOWNLOAD_URL}"| tar xz -C "${INSTALL_PATH}"
    chmod +x "${INSTALL_PATH}"/ibmcloud
  fi
}