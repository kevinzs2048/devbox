package main

import (
  "fmt"
  "github.com/gophercloud/gophercloud"
  "github.com/gophercloud/gophercloud/openstack"
  "github.com/gophercloud/gophercloud/openstack/container/experimental/capsules"
)

func main() {
        opts := gophercloud.AuthOptions{
		IdentityEndpoint: "http://10.169.41.188/identity",
		Username: "admin",
		Password: "password",
		TenantID: "279b987f9ea2449b9f98fd94fb700fd8",
		DomainID: "default",
	}
	provider, err := openstack.AuthenticatedClient(opts)
	fmt.Println(err)
	if err != nil {
		fmt.Errorf("Sending get container group request failed: %v", err)
		return
	}
	fmt.Println(opts)
	client, err := openstack.NewContainerExperimental(provider, gophercloud.EndpointOpts{
		Region: "RegionOne",
	})
        if err != nil {
		fmt.Errorf("Get zun client failed: %v", err)
		return
	}
	fmt.Println(opts)
	ret := capsules.Get(client, "e6c913bb-b4e4-409d-8b71-3e029f196458")
	server, id := ret.Extract()
	fmt.Println("====================")
	fmt.Println(id)
	fmt.Println(ret)
	fmt.Println("====================")
	fmt.Println(server)
}

