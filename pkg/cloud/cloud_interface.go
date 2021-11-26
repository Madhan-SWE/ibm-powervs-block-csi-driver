package cloud

type Cloud interface {
	CreateDisk(volumeName string, diskOptions *DiskOptions) (disk *Disk, err error)
	DeleteDisk(volumeID string) (success bool, err error)
	AttachDisk(volumeID string, nodeID string) (err error)
	DetachDisk(volumeID string, nodeID string) (err error)
	ResizeDisk(volumeID string, reqSize int64) (newSize int64, err error)
	WaitForAttachmentState(volumeID, state string) error
	GetDiskByName(name string) (disk *Disk, err error)
	GetDiskByID(volumeID string) (disk *Disk, err error)
	GetPVMInstanceByName(instanceName string) (instance *PVMInstance, err error)
	GetPVMInstanceByID(instanceID string) (instance *PVMInstance, err error)
	GetImageByID(imageID string) (image *PVMImage, err error)
	IsAttached(volumeID string, nodeID string) (attached bool, err error)
}
