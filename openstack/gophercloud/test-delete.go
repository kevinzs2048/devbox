package main

import (
  "fmt"
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
	err = capsules.Delete(client, "e64c25a4-ff62-4bce-8442-f1cf78d6f053").ExtractErr()
	if err != nil {
		fmt.Errorf("Failed to delete: %v", err)
		fmt.Printf("Failed to delete: %v", err)
		return
	}
}

