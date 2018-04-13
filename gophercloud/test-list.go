package main

import (
  "fmt"
  "encoding/json"

  "github.com/gophercloud/gophercloud"
  "github.com/gophercloud/gophercloud/openstack"
  "github.com/gophercloud/gophercloud/openstack/container/v1/capsules"
)

// TenantID is the project ID of your user(if your user is admin, then choose the admin user ID)
func main() {
        opts := gophercloud.AuthOptions{
		IdentityEndpoint: "http://10.169.41.188/identity",
		Username: "admin",
		Password: "password",
		TenantID: "01821bd38f2f474489491adb0da7efaf",
		DomainID: "default",
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		fmt.Errorf("Sending get container group request failed: %v", err)
		return
	}
	client, err := openstack.NewContainerV1(provider, gophercloud.EndpointOpts{
		Region: "RegionOne",
	})
        if err != nil {
		fmt.Errorf("Get zun client failed: %v", err)
		return
	}
	allPages, err := capsules.List(client, nil).AllPages()
	if err != nil {
                fmt.Printf("===============")
                fmt.Printf("============%s===\r\n", err)
		fmt.Errorf("Unable to retrieve capsules: %v", err)
                return
	}
	allCapsules, err := capsules.ExtractCapsules(allPages)
	if err != nil {
		fmt.Errorf("Unable to extract capsules: %v", err)
                return
	}

	for _, capsule := range allCapsules {
	        b, _ := json.MarshalIndent(capsule, "", "  ")
                fmt.Printf("%s", string(b))
        }
}

