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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/ppc64le-cloud/powervs-csi-driver/pkg/cloud"
	cloudmocks "github.com/ppc64le-cloud/powervs-csi-driver/pkg/cloud/mocks"
	mocks "github.com/ppc64le-cloud/powervs-csi-driver/pkg/driver/mocks"

	// "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	volumeID = "voltest"
	nvmeName = "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_voltest"
	// symlinkFileInfo = fs.FileInfo(&fakeFileInfo{nvmeName, os.ModeSymlink})
)

func TestNodeStageVolume(t *testing.T) {

	var (
		targetPath = "/test/path"
		devicePath = "/dev/fake"
		// source     = "source"
		// nvmeDevicePath = "/dev/fakenvme1n1"

		stdVolCap = &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{
					FsType: FSTypeExt4,
				},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}

		stdVolContext = map[string]string{VolumeAttributePartition: "1"}
		// devicePathWithPartition = devicePath + "1"
		// With few exceptions, all "success" non-block cases have roughly the same
		// expected calls and only care about testing the FormatAndMount call. The
		// exceptions should not call this, instead they should define expectMock
		// from scratch.

		successExpectMock = func(mockMounter mocks.MockMounter) {
			mockMounter.EXPECT().RescanSCSIBus().Return(nil)
			mockMounter.EXPECT().GetDevicePath(gomock.Eq(devicePath)).Return(devicePath, nil)
			mockMounter.EXPECT().ExistsPath(gomock.Eq(targetPath)).Return(false, nil)
			mockMounter.EXPECT().MakeDir(gomock.Any()).Return(nil)
			mockMounter.EXPECT().GetDeviceName(gomock.Eq(targetPath)).Return(targetPath, 1, nil)

		}
	)
	testCases := []struct {
		name         string
		request      *csi.NodeStageVolumeRequest
		expectMock   func(mockMounter mocks.MockMounter)
		expectedCode codes.Code
	}{

		{
			name: "success normal",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeId:          volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				successExpectMock(mockMounter)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Any(), gomock.Any()).Return(nil)
			},
		},

		{
			name: "success normal [raw block]",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				mockMounter.EXPECT().FormatAndMount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
		},

		{
			name: "success with mount options",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: []string{"dirsync", "noexec"},
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				successExpectMock(mockMounter)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt4), gomock.Eq([]string{"dirsync", "noexec"}))
			},
		},

		{
			name: "success fsType ext3",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: FSTypeExt3,
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				successExpectMock(mockMounter)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt3), gomock.Any())
			},
		},

		{
			name: "success mount with default fsType ext4",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				successExpectMock(mockMounter)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt4), gomock.Any())
			},
		},

		{
			name: "success device already mounted at target",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeId:          volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				mockMounter.EXPECT().RescanSCSIBus().Return(nil)
				mockMounter.EXPECT().GetDevicePath(gomock.Eq(devicePath)).Return(devicePath, nil)
				mockMounter.EXPECT().ExistsPath(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().GetDeviceName(gomock.Eq(targetPath)).Return(devicePath, 1, nil)

				mockMounter.EXPECT().FormatAndMount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
		},

		{
			name: "success with partition",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeContext:     stdVolContext,
				VolumeId:          volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				successExpectMock(mockMounter)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(DefaultFsType), gomock.Any())
			},
		},
		{
			name: "success with invalid partition config, will ignore partition",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeContext:     map[string]string{VolumeAttributePartition: "0"},
				VolumeId:          volumeID,
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				successExpectMock(mockMounter)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(DefaultFsType), gomock.Any())
			},
		},

		{
			name: "fail no VolumeId",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no mount",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no StagingTargetPath",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:   map[string]string{WWNKey: devicePath},
				VolumeCapability: stdVolCap,
				VolumeId:         volumeID,
			},
			expectedCode: codes.InvalidArgument,
		},

		{
			name: "fail no VolumeCapability",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeId:          volumeID,
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail invalid VolumeCapability",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{WWNKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
				VolumeId: volumeID,
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no devicePath",
			request: &csi.NodeStageVolumeRequest{
				VolumeCapability: stdVolCap,
				VolumeId:         volumeID,
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail invalid volumeContext",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeContext:     map[string]string{VolumeAttributePartition: "partition1"},
				VolumeId:          volumeID,
			},
			expectedCode: codes.InvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMounter := mocks.NewMockMounter(mockCtl)

			powervsDriver := &nodeService{
				mounter: mockMounter,
			}

			if tc.expectMock != nil {
				tc.expectMock(*mockMounter)
			}

			_, err := powervsDriver.NodeStageVolume(context.TODO(), tc.request)
			fmt.Printf("Request: %+v", tc.request)
			fmt.Printf("Error: %+v", err)
			if tc.expectedCode != codes.OK {
				expectErr(t, err, tc.expectedCode)
			} else if err != nil {
				t.Fatalf("Expect no error but got: %v", err)
			}
		})
	}
}

func TestNodeExpandVolume(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockMounter := mocks.NewMockMounter(mockCtl)

	powervsDriver := &nodeService{
		mounter: mockMounter,
	}

	tests := []struct {
		name               string
		request            csi.NodeExpandVolumeRequest
		expectResponseCode codes.Code
		expectMock         func(mockMounter mocks.MockMounter)
	}{
		{
			name:               "fail missing volumeId",
			request:            csi.NodeExpandVolumeRequest{},
			expectResponseCode: codes.InvalidArgument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.expectMock != nil {
				test.expectMock(*mockMounter)
			}
			_, err := powervsDriver.NodeExpandVolume(context.Background(), &test.request)
			if err != nil {
				if test.expectResponseCode != codes.OK {
					expectErr(t, err, test.expectResponseCode)
				} else {
					t.Fatalf("Expect no error but got: %v", err)
				}
			}
		})
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	targetPath := "/test/path"

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMounter := mocks.NewMockMounter(mockCtl)

				powervsDriver := &nodeService{
					mounter: mockMounter,
				}

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
					VolumeId:   "vol-test",
				}

				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)
				_, err := powervsDriver.NodeUnpublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},

		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMounter := mocks.NewMockMounter(mockCtl)

				powervsDriver := &nodeService{
					mounter: mockMounter,
				}

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
				}

				_, err := powervsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail no TargetPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMounter := mocks.NewMockMounter(mockCtl)

				powervsDriver := &nodeService{
					mounter: mockMounter,
				}

				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId: "vol-test",
				}

				_, err := powervsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail error on unmount",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMounter := mocks.NewMockMounter(mockCtl)

				powervsDriver := &nodeService{
					mounter: mockMounter,
				}

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
					VolumeId:   "vol-test",
				}

				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(errors.New("test Unmount error"))
				_, err := powervsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeGetCapabilities(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockMounter := mocks.NewMockMounter(mockCtl)

	powervsDriver := nodeService{
		mounter: mockMounter,
	}

	caps := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
				},
			},
		},
	}
	expResp := &csi.NodeGetCapabilitiesResponse{Capabilities: caps}

	req := &csi.NodeGetCapabilitiesRequest{}
	resp, err := powervsDriver.NodeGetCapabilities(context.TODO(), req)
	if err != nil {
		srvErr, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Could not get error status code from error: %v", srvErr)
		}
		t.Fatalf("Expected nil error, got %d message %s", srvErr.Code(), srvErr.Message())
	}
	if !reflect.DeepEqual(expResp, resp) {
		t.Fatalf("Expected response {%+v}, got {%+v}", expResp, resp)
	}
}

func TestNodeGetInfo(t *testing.T) {
	testCases := []struct {
		name              string
		instanceID        string
		instanceType      string
		availabilityZone  string
		volumeAttachLimit int64
		expMaxVolumes     int64
	}{
		{
			name:              "success normal",
			instanceID:        "i-123456789abcdef01",
			instanceType:      "t2.medium",
			availabilityZone:  "us-west-2b",
			volumeAttachLimit: 30,
			expMaxVolumes:     30,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			driverOptions := &Options{
				volumeAttachLimit: tc.volumeAttachLimit,
			}

			mockMounter := mocks.NewMockMounter(mockCtl)
			mockCloud := cloudmocks.NewMockCloud(mockCtl)

			mockCloud.EXPECT().GetPVMInstanceByName(gomock.Any()).Return(&cloud.PVMInstance{
				ID:      tc.instanceID,
				Name:    tc.name,
				ImageID: "test-image",
			}, nil)

			mockCloud.EXPECT().GetImageByID(gomock.Eq("test-image")).Return(&cloud.PVMImage{
				ID:       "test-image",
				Name:     "test-image",
				DiskType: "tier3",
			}, nil)

			powervsDriver := &nodeService{
				mounter:       mockMounter,
				driverOptions: driverOptions,
				cloud:         mockCloud,
			}

			resp, err := powervsDriver.NodeGetInfo(context.TODO(), &csi.NodeGetInfoRequest{})
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				t.Fatalf("Expected nil error, got %d message %s", srvErr.Code(), srvErr.Message())
			}

			if resp.GetNodeId() != tc.instanceID {
				t.Fatalf("Expected node ID %q, got %q", tc.instanceID, resp.GetNodeId())
			}

			if resp.GetMaxVolumesPerNode() != tc.expMaxVolumes {
				t.Fatalf("Expected %d max volumes per node, got %d", tc.expMaxVolumes, resp.GetMaxVolumesPerNode())
			}

		})
	}
}

func expectErr(t *testing.T, actualErr error, expectedCode codes.Code) {
	if actualErr == nil {
		t.Fatalf("Expect error but got no error")
	}

	status, ok := status.FromError(actualErr)
	if !ok {
		t.Fatalf("Failed to get error status code from error: %v", actualErr)
	}

	if status.Code() != expectedCode {
		t.Fatalf("Expected error code %d, got %d message %s", codes.InvalidArgument, status.Code(), status.Message())
	}
}
