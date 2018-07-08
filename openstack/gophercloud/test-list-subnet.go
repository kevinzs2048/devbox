package main

import (
  "encoding/json"
  "fmt"
  "github.com/gophercloud/gophercloud"
  "github.com/gophercloud/gophercloud/openstack"
  "github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
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
	fmt.Println(err)
	if err != nil {
		fmt.Errorf("Sending get container group request failed: %v", err)
		return
	}
        client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: "RegionOne",
	})
	if err != nil {
		fmt.Errorf("Unable to create a network client: %v", err)
	}
	allPages, err := subnets.List(client, nil).AllPages()
	if err != nil {
		fmt.Errorf("Unable to list subnets: %v", err)
	}

	allSubnets, err := subnets.ExtractSubnets(allPages)
	if err != nil {
		fmt.Errorf("Unable to extract subnets: %v", err)
	}

	for _, subnet := range allSubnets {
	        b, _ := json.MarshalIndent(subnet, "", "  ")
                fmt.Printf("%s", string(b))
        }

}

