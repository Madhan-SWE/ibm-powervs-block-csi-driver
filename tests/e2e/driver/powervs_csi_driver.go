package driver

import (
	"fmt"

	powervscsidriver "github.com/ppc64le-cloud/powervs-csi-driver/pkg/driver"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	True = "true"
)

// Implement DynamicPVTestDriver interface
type powervsCSIDriver struct {
	driverName string
}

// InitPowervsCSIDriver returns powervsCSIDriver that implements DynamicPVTestDriver interface
func InitPowervsCSIDriver() PVTestDriver {
	return &powervsCSIDriver{
		driverName: powervscsidriver.DriverName,
	}
}

func (d *powervsCSIDriver) GetDynamicProvisionStorageClass(parameters map[string]string, mountOptions []string, reclaimPolicy *v1.PersistentVolumeReclaimPolicy, volumeExpansion *bool, bindingMode *storagev1.VolumeBindingMode, allowedTopologyValues []string, namespace string) *storagev1.StorageClass {
	provisioner := d.driverName
	generateName := fmt.Sprintf("%s-%s-dynamic-sc-", namespace, provisioner)
	allowedTopologies := []v1.TopologySelectorTerm{}

	if len(allowedTopologyValues) > 0 {
		allowedTopologies = []v1.TopologySelectorTerm{
			{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
					{
						// TODO we should use the new topology key eventually
						Key:    powervscsidriver.TopologyKey,
						Values: allowedTopologyValues,
					},
				},
			},
		}
	}
	return getStorageClass(generateName, provisioner, parameters, mountOptions, reclaimPolicy, volumeExpansion, bindingMode, allowedTopologies)
}

func (d *powervsCSIDriver) GetPersistentVolume(volumeID string, fsType string, size string, reclaimPolicy *v1.PersistentVolumeReclaimPolicy, namespace string) *v1.PersistentVolume {
	fmt.Println("Get Persistent Volume", " = provisioner = ", d.driverName)
	provisioner := d.driverName
	generateName := fmt.Sprintf("%s-%s-preprovsioned-pv-", namespace, provisioner)
	// Default to Retain ReclaimPolicy for pre-provisioned volumes
	pvReclaimPolicy := v1.PersistentVolumeReclaimRetain
	if reclaimPolicy != nil {
		pvReclaimPolicy = *reclaimPolicy
	}
	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    namespace,
			// TODO remove if https://github.com/kubernetes-csi/external-provisioner/issues/202 is fixed
			Annotations: map[string]string{
				"pv.kubernetes.io/provisioned-by": provisioner,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse(size),
			},
			PersistentVolumeReclaimPolicy: pvReclaimPolicy,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       provisioner,
					VolumeHandle: volumeID,
					FSType:       fsType,
				},
			},
		},
	}
}

// GetParameters returns the parameters specific for this driver
func GetParameters(volumeType string, fsType string, encrypted bool) map[string]string {
	parameters := map[string]string{
		"type":                      volumeType,
		"csi.storage.k8s.io/fstype": fsType,
	}
	if iopsPerGB := IOPSPerGBForVolumeType(volumeType); iopsPerGB != "" {
		parameters[powervscsidriver.IopsPerGBKey] = iopsPerGB
	}
	if iops := IOPSForVolumeType(volumeType); iops != "" {
		parameters[powervscsidriver.IopsKey] = iops
	}
	if throughput := ThroughputForVolumeType(volumeType); throughput != "" {
		parameters[powervscsidriver.ThroughputKey] = throughput
	}
	if encrypted {
		parameters[powervscsidriver.EncryptedKey] = True
	}
	return parameters
}

// IOPSPerGBForVolumeType returns the maximum iops per GB for each volumeType
// Otherwise returns an empty string
func IOPSPerGBForVolumeType(volumeType string) string {
	switch volumeType {
	// case "tier1":
	// 	// Maximum IOPS/GB for io1 is 50
	// 	return "10"
	// case "tier3":
	// 	// Maximum IOPS/GB for io2 is 500
	// 	return "3"
	default:
		return ""
	}
}

// IOPSForVolumeType returns the maximum iops for each volumeType
// Otherwise returns an empty string
func IOPSForVolumeType(volumeType string) string {
	switch volumeType {
	case "gp3":
		// Maximum IOPS for gp3 is 16000. However, maximum IOPS/GB for gp3 is 500.
		// Since the tests will run using minimum volume capacity (1GB), set to 500.
		return "500"
	default:
		return ""
	}
}

// ThroughputPerVolumeType returns the maximum throughput for each volumeType
// Otherwise returns an empty string
func ThroughputForVolumeType(volumeType string) string {
	switch volumeType {
	case "gp3":
		// Maximum throughput for gp3 is 1000. However, maximum throughput/iops for gp3 is 0.25
		// Since the default iops is 3000, set to 750.
		return "750"
	default:
		return ""
	}
}
