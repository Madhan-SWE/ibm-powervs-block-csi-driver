package cloud

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

// Metadata is info about the instance on which the driver is running
type Metadata struct {
	serviceInstanceID string
}

var _ MetadataService = &Metadata{}

// GetServiceInstanceId returns service instance id of the instance
func (m *Metadata) GetServiceInstanceId() string {
	return m.serviceInstanceID
}

type KubernetesAPIClient func() (kubernetes.Interface, error)

var DefaultKubernetesAPIClient = func() (kubernetes.Interface, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func NewMetadataService(k8sAPIClient KubernetesAPIClient) (MetadataService, error) {

	klog.Infof("retrieving instance data from kubernetes api")
	clientset, err := k8sAPIClient()
	if err != nil {
		klog.Warningf("error creating kubernetes api client: %v", err)
	} else {
		klog.Infof("kubernetes api is available")
		return KubernetesAPIInstanceInfo(clientset)
	}

	return nil, fmt.Errorf("error getting instance data from ec2 metadata or kubernetes api")
}

func KubernetesAPIInstanceInfo(clientset kubernetes.Interface) (*Metadata, error) {
	nodeName := os.Getenv("CSI_NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("CSI_NODE_NAME env var not set")
	}

	// get node with k8s API
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Node %v: %v", nodeName, err)
	}
	klog.Infoln("#####################################################################################")
	klog.Infoln("node details : %+v", node)
	klog.Infoln("#####################################################################################")

	it := node.GetLabels()
	klog.Infoln("#####################################################################################")
	klog.Infoln("Instance labels: %+v", it)
	klog.Infoln("#####################################################################################")

	return nil, fmt.Errorf("error getting Node %v", nodeName)
}
