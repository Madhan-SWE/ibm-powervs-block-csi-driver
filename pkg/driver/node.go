/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ppc64le-cloud/powervs-csi-driver/pkg/cloud"
	"github.com/ppc64le-cloud/powervs-csi-driver/pkg/fibrechannel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mountutils "k8s.io/mount-utils"
)

const (

	// default file system type to be used when it is not provided
	defaultFsType = "ext4"

	// defaultMaxVolumesPerInstance is the limit of volumes can be attached in the PowerVS environment
	// TODO: rightnow 99 is just a placeholder, this needs to be changed post discussion with PowerVS team
	defaultMaxVolumesPerInstance = 127 - 1
)

var (
	// nodeCaps represents the capability of node service.
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	}
)

// nodeService represents the node service of CSI driver
type nodeService struct {
	cloud         cloud.Cloud
	mounter       Mounter
	driverOptions *Options
}

// newNodeService creates a new node service
// it panics if failed to create the service
func newNodeService(driverOptions *Options) nodeService {
	klog.V(4).Infof("retrieving node info from metadata service")
	metadata, err := cloud.NewMetadataService(cloud.DefaultKubernetesAPIClient)
	if err != nil {
		panic(err)
	}

	pvsCloud, err := NewPowerVSCloudFunc(metadata.GetCloudInstanceId(), driverOptions.debug)
	if err != nil {
		panic(err)
	}

	return nodeService{
		cloud:         pvsCloud,
		mounter:       newNodeMounter(),
		driverOptions: driverOptions,
	}
}

func (d *nodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("NodeStageVolume: called with args %+v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	// If the access type is block, do nothing for stage
	switch volCap.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		return &csi.NodeStageVolumeResponse{}, nil
	}

	mnt := volCap.GetMount()
	if mnt == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume: mnt is nil within volume capability")
	}

	fsType := mnt.GetFsType()
	if len(fsType) == 0 {
		fsType = defaultFsType
	}

	var mountOptions []string
	for _, f := range mnt.MountFlags {
		if !hasMountOption(mountOptions, f) {
			mountOptions = append(mountOptions, f)
		}
	}

	wwn, ok := req.PublishContext[WWNKey]
	if !ok || wwn == "" {
		return nil, status.Error(codes.InvalidArgument, "WWN ID is not provided or empty")
	}

	_ = d.mounter.RescanSCSIBus()

	source, err := d.mounter.GetDevicePath(wwn)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to find device path %s. %v", wwn, err)
	}

	klog.V(4).Infof("NodeStageVolume: find device path for wwn %s -> %s", wwn, source)

	exists, err := d.mounter.ExistsPath(target)
	if err != nil {
		msg := fmt.Sprintf("failed to check if target %q exists: %v", target, err)
		return nil, status.Error(codes.Internal, msg)
	}
	// When exists is true it means target path was created but device isn't mounted.
	// We don't want to do anything in that case and let the operation proceed.
	// Otherwise we need to create the target directory.
	if !exists {
		// If target path does not exist we need to create the directory where volume will be staged
		klog.V(4).Infof("NodeStageVolume: creating target dir %q", target)
		if err = d.mounter.MakeDir(target); err != nil {
			msg := fmt.Sprintf("could not create target dir %q: %v", target, err)
			return nil, status.Error(codes.Internal, msg)
		}
	}

	// Check if a device is mounted in target directory
	device, _, err := d.mounter.GetDeviceName(target)
	if err != nil {
		msg := fmt.Sprintf("failed to check if volume is already mounted: %v", err)
		return nil, status.Error(codes.Internal, msg)
	}

	// This operation (NodeStageVolume) MUST be idempotent.
	// If the volume corresponding to the volume_id is already staged to the staging_target_path,
	// and is identical to the specified volume_capability the Plugin MUST reply 0 OK.
	if device == source {
		klog.V(4).Infof("NodeStageVolume: volume=%q already staged", volumeID)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// FormatAndMount will format only if needed
	klog.V(5).Infof("NodeStageVolume: formatting %s and mounting at %s with fstype %s", source, target, fsType)
	err = d.mounter.FormatAndMount(source, target, fsType, mountOptions)
	if err != nil {
		msg := fmt.Sprintf("could not format %q and mnt it at %q", source, target)
		return nil, status.Error(codes.Internal, msg)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *nodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(4).Infof("NodeUnstageVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	// Check if target directory is a mount point. GetDeviceNameFromMount
	// given a mnt point, finds the device from /proc/mounts
	// returns the device name, reference count, and error code
	dev, refCount, err := d.mounter.GetDeviceName(target)
	if err != nil {
		msg := fmt.Sprintf("failed to check if volume is mounted: %v", err)
		return nil, status.Error(codes.Internal, msg)
	}

	// From the spec: If the volume corresponding to the volume_id
	// is not staged to the staging_target_path, the Plugin MUST
	// reply 0 OK.
	if refCount == 0 {
		klog.V(5).Infof("NodeUnstageVolume: %s target not mounted", target)
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if refCount > 1 {
		klog.Warningf("NodeUnstageVolume: found %d references to device %s mounted at target path %s", refCount, dev, target)
	}

	klog.V(5).Infof("NodeUnstageVolume: unmounting %s", target)
	err = d.mounter.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmount target %q: %v", target, err)
	}
	handler := &fibrechannel.OSioHandler{}
	var mpath bool
	if mdev, _ := fibrechannel.FindMultipathDeviceForDevice(dev, handler); mdev != "" {
		klog.V(5).Infof("Multipath device found: %s for %s", mdev, dev)
		mpath = true
		dev = mdev
	}
	klog.Infof("Detaching: %s", dev)
	err = fibrechannel.Detach(dev, handler)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to detach %s: %v", dev, err)
	}
	if mpath {
		klog.Infof("Deleting the multipath device: %s", dev)
		if err := fibrechannel.RemoveMultipathDevice(dev); err != nil {
			return nil, err
		}
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).Infof("NodeExpandVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	args := []string{"-o", "source", "--noheadings", "--target", req.GetVolumePath()}
	output, err := d.mounter.Command("findmnt", args...).Output()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not determine device path: %v", err)

	}

	devicePath := strings.TrimSpace(string(output))
	if len(devicePath) == 0 {
		return nil, status.Errorf(codes.Internal, "Could not get valid device for mount path: %q", req.GetVolumePath())
	}

	// TODO: refactor Mounter to expose a mount.SafeFormatAndMount object
	r := mountutils.NewResizeFs(d.mounter.(*NodeMounter).Exec)

	// TODO: lock per volume ID to have some idempotency
	if _, err := r.Resize(devicePath, req.GetVolumePath()); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not resize volume %q (%q):  %v", volumeID, devicePath, err)
	}

	return &csi.NodeExpandVolumeResponse{}, nil
}

func (d *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("NodePublishVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	source := req.GetStagingTargetPath()
	if len(source) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	mountOptions := []string{"bind"}
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	switch mode := volCap.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		if err := d.nodePublishVolumeForBlock(req, mountOptions); err != nil {
			return nil, err
		}
	case *csi.VolumeCapability_Mount:
		if err := d.nodePublishVolumeForFileSystem(req, mountOptions, mode); err != nil {
			return nil, err
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	klog.V(5).Infof("NodeUnpublishVolume: unmounting %s", target)
	err := d.mounter.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeGetVolumeStats is not implemented yet")
}

func (d *nodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("NodeGetCapabilities: called with args %+v", *req)
	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeCaps {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (d *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: called with args %+v", *req)
	klog.V(4).Infof("retrieving node info from metadata service")
	metadata, err := cloud.NewMetadataService(cloud.DefaultKubernetesAPIClient)
	if err != nil {
		panic(err)
	}
	pvmInstanceId := metadata.GetPvmInstanceId()

	in, err := d.cloud.GetPVMInstanceByID(pvmInstanceId)
	if err != nil {
		klog.Errorf("failed to get the instance for pvmInstanceId %s, err: %s", pvmInstanceId, err)
		return nil, fmt.Errorf("failed to get the instance for pvmInstanceId %s, err: %s", pvmInstanceId, err)
	}
	image, err := d.cloud.GetImageByID(in.ImageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get the image details for %s, err: %s", in.ImageID, err)
	}

	segments := map[string]string{
		DiskTypeKey: image.DiskType,
	}

	topology := &csi.Topology{Segments: segments}

	return &csi.NodeGetInfoResponse{
		NodeId:             pvmInstanceId,
		MaxVolumesPerNode:  d.getVolumesLimit(),
		AccessibleTopology: topology,
	}, nil
}

func (d *nodeService) nodePublishVolumeForBlock(req *csi.NodePublishVolumeRequest, mountOptions []string) error {
	target := req.GetTargetPath()
	//volumeID := req.GetVolumeId()

	wwn, exists := req.PublishContext[WWNKey]
	if !exists {
		return status.Error(codes.InvalidArgument, "WWN ID not provided")
	}
	source, err := d.mounter.GetDevicePath(wwn)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to find device path for wwn %s. %v", wwn, err)
	}

	klog.V(4).Infof("NodePublishVolume [block]: find device path for wwn %s -> %s", wwn, source)

	globalMountPath := filepath.Dir(target)

	// create the global mount path if it is missing
	// Path in the form of /var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/publish/{volumeName}
	exists, err = d.mounter.ExistsPath(globalMountPath)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not check if path exists %q: %v", globalMountPath, err)
	}

	if !exists {
		if err = d.mounter.MakeDir(globalMountPath); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", globalMountPath, err)
		}
	}

	// Create the mount point as a file since bind mount device node requires it to be a file
	klog.V(5).Infof("NodePublishVolume [block]: making target file %s", target)
	err = d.mounter.MakeFile(target)
	if err != nil {
		if removeErr := os.Remove(target); removeErr != nil {
			return status.Errorf(codes.Internal, "Could not remove mount target %q: %v", target, removeErr)
		}
		return status.Errorf(codes.Internal, "Could not create file %q: %v", target, err)
	}

	klog.V(5).Infof("NodePublishVolume [block]: mounting %s at %s", source, target)
	if err := d.mounter.Mount(source, target, "", mountOptions); err != nil {
		if removeErr := os.Remove(target); removeErr != nil {
			return status.Errorf(codes.Internal, "Could not remove mount target %q: %v", target, removeErr)
		}
		return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
	}

	return nil
}

func (d *nodeService) nodePublishVolumeForFileSystem(req *csi.NodePublishVolumeRequest, mountOptions []string, mode *csi.VolumeCapability_Mount) error {
	target := req.GetTargetPath()
	source := req.GetStagingTargetPath()
	if m := mode.Mount; m != nil {
		for _, f := range m.MountFlags {
			if !hasMountOption(mountOptions, f) {
				mountOptions = append(mountOptions, f)
			}
		}
	}

	klog.V(5).Infof("NodePublishVolume: creating dir %s", target)
	if err := d.mounter.MakeDir(target); err != nil {
		return status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	fsType := mode.Mount.GetFsType()
	if len(fsType) == 0 {
		fsType = defaultFsType
	}

	klog.V(5).Infof("NodePublishVolume: mounting %s at %s with option %s as fstype %s", source, target, mountOptions, fsType)
	if err := d.mounter.Mount(source, target, fsType, mountOptions); err != nil {
		if removeErr := os.Remove(target); removeErr != nil {
			return status.Errorf(codes.Internal, "Could not remove mount target %q: %v", target, err)
		}
		return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
	}

	return nil
}

// getVolumesLimit returns the limit of volumes that the node supports
func (d *nodeService) getVolumesLimit() int64 {
	if d.driverOptions.volumeAttachLimit >= 0 {
		return d.driverOptions.volumeAttachLimit
	}
	return defaultMaxVolumesPerInstance
}

// hasMountOption returns a boolean indicating whether the given
// slice already contains a mount option. This is used to prevent
// passing duplicate option to the mount command.
func hasMountOption(options []string, opt string) bool {
	for _, o := range options {
		if o == opt {
			return true
		}
	}
	return false
}
