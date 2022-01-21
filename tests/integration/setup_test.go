/*
Copyright 2022 The Kubernetes Authors.

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

package integration

import (
	"context"
	"flag"
	"fmt"
	"net"
	gohttp "net/http"
	"os"
	"strings"
	"testing"

	"github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/api/resource/resourcev2/controllerv2"
	"github.com/IBM-Cloud/bluemix-go/authentication"
	"github.com/IBM-Cloud/bluemix-go/http"
	"github.com/IBM-Cloud/bluemix-go/rest"
	bxsession "github.com/IBM-Cloud/bluemix-go/session"
	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang-jwt/jwt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"sigs.k8s.io/ibm-powervs-block-csi-driver/pkg/cloud"
	"sigs.k8s.io/ibm-powervs-block-csi-driver/pkg/driver"
	"sigs.k8s.io/ibm-powervs-block-csi-driver/pkg/util"
)

const (
	endpoint = "tcp://127.0.0.1:10000"
)

var (
	drv       *driver.Driver
	csiClient *CSIClient
	volClient *instance.IBMPIVolumeClient
)

type User struct {
	ID         string
	Email      string
	Account    string
	cloudName  string `default:"bluemix"`
	cloudType  string `default:"public"`
	generation int    `default:"2"`
}

func TestIntegration(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS EBS CSI Driver Integration Tests")
}

var _ = BeforeSuite(func() {
	// Run CSI Driver in its own goroutine
	var err error
	drv, err = driver.NewDriver(driver.WithEndpoint(endpoint))
	Expect(err).To(BeNil())
	go func() {
		err = drv.Run()
		Expect(err).To(BeNil())
	}()

	// Create CSI Controller client
	csiClient, err = newCSIClient()
	Expect(err).To(BeNil(), "Set up Controller Client failed with error")
	Expect(csiClient).NotTo(BeNil())

	// Create Volume client
	volClient, err = newVolClient()
	Expect(err).To(BeNil(), "Set up  volClient failed with error")
	Expect(volClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	drv.Stop()
})

type CSIClient struct {
	ctrl csi.ControllerClient
	node csi.NodeClient
}

func newCSIClient() (*CSIClient, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
				scheme, addr, err := util.ParseEndpoint(endpoint)
				if err != nil {
					return nil, err
				}
				return net.Dial(scheme, addr)
			}),
	}
	grpcClient, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return &CSIClient{
		ctrl: csi.NewControllerClient(grpcClient),
		node: csi.NewNodeClient(grpcClient),
	}, nil
}

func newMetadata() (cloud.MetadataService, error) {
	metadata, err := cloud.NewMetadataService(cloud.DefaultKubernetesAPIClient)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

func newVolClient() (*instance.IBMPIVolumeClient, error) {

	apikey := os.Getenv("IBMCLOUD_API_KEY")
	bxSess, err := bxsession.New(&bluemix.Config{BluemixAPIKey: apikey})
	if err != nil {
		return nil, err
	}

	err = authenticateAPIKey(bxSess)
	if err != nil {
		return nil, err
	}

	user, err := fetchUserDetails(bxSess, 2)
	if err != nil {
		return nil, err
	}

	ctrlv2, err := controllerv2.New(bxSess)
	if err != nil {
		return nil, err
	}

	metadata, err := newMetadata()
	if err != nil {
		return nil, err
	}

	cloudInstanceID := metadata.GetCloudInstanceId()
	resourceClient := ctrlv2.ResourceServiceInstanceV2()
	in, err := resourceClient.GetInstance(cloudInstanceID)
	if err != nil {
		return nil, err
	}

	zone := in.RegionID
	region, err := getRegion(zone)
	if err != nil {
		return nil, err
	}

	piSession, err := ibmpisession.New(bxSess.Config.IAMAccessToken, region, true, user.Account, zone)
	if err != nil {
		return nil, err
	}

	backgroundContext := context.Background()
	volClient := instance.NewIBMPIVolumeClient(backgroundContext, piSession, cloudInstanceID)

	return volClient, nil

}

func logf(format string, args ...interface{}) {
	fmt.Fprintln(GinkgoWriter, fmt.Sprintf(format, args...))
}

func authenticateAPIKey(sess *bxsession.Session) error {
	config := sess.Config
	tokenRefresher, err := authentication.NewIAMAuthRepository(config, &rest.Client{
		DefaultHeader: gohttp.Header{
			"User-Agent": []string{http.UserAgent()},
		},
	})
	if err != nil {
		return err
	}
	return tokenRefresher.AuthenticateAPIKey(config.BluemixAPIKey)
}

func fetchUserDetails(sess *bxsession.Session, generation int) (*User, error) {
	config := sess.Config
	user := User{}
	var bluemixToken string

	if strings.HasPrefix(config.IAMAccessToken, "Bearer") {
		bluemixToken = config.IAMAccessToken[7:len(config.IAMAccessToken)]
	} else {
		bluemixToken = config.IAMAccessToken
	}

	token, err := jwt.Parse(bluemixToken, func(token *jwt.Token) (interface{}, error) {
		return "", nil
	})
	if err != nil && !strings.Contains(err.Error(), "key is of invalid type") {
		return &user, err
	}

	claims := token.Claims.(jwt.MapClaims)
	if email, ok := claims["email"]; ok {
		user.Email = email.(string)
	}
	user.ID = claims["id"].(string)
	user.Account = claims["account"].(map[string]interface{})["bss"].(string)
	iss := claims["iss"].(string)
	if strings.Contains(iss, "https://iam.cloud.ibm.com") {
		user.cloudName = "bluemix"
	} else {
		user.cloudName = "staging"
	}
	user.cloudType = "public"

	user.generation = generation
	return &user, nil
}

func getRegion(zone string) (region string, err error) {
	err = nil
	switch {
	case strings.HasPrefix(zone, "us-south"):
		region = "us-south"
	case strings.HasPrefix(zone, "us-east"):
		region = "us-east"
	case strings.HasPrefix(zone, "tor"):
		region = "tor"
	case strings.HasPrefix(zone, "eu-de-"):
		region = "eu-de"
	case strings.HasPrefix(zone, "lon"):
		region = "lon"
	case strings.HasPrefix(zone, "syd"):
		region = "syd"
	default:
		return "", fmt.Errorf("region not found for the zone, talk to the developer to add the support into the tool: %s", zone)
	}
	return
}
