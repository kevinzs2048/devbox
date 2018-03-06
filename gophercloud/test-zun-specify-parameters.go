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
		TenantID: "4036ad025daf4fcbb2c0cde3d6e59073",
		DomainID: "default",
	}
	provider, err := openstack.AuthenticatedClient(opts)
	fmt.Println(err)
	if err != nil {
		fmt.Errorf("Sending get container group request failed: %v", err)
		return
	}
	fmt.Println(opts)
	client, err := openstack.NewContainerV1(provider, gophercloud.EndpointOpts{
		Region: "RegionOne",
	})
        if err != nil {
		fmt.Errorf("Get zun client failed: %v", err)
		return
	}
	fmt.Println(opts)
	ret := capsules.Get(client, "871966da-386d-4a76-bc2a-3cd0032dc267")
	server, id := ret.Extract()
	fmt.Println("====================")
	fmt.Println(id)
	fmt.Println(ret)
	fmt.Println("====================")
	fmt.Println(server)
}

