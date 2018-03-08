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
        template := new(capsules.Template)
	template.Bin = []byte(`{"kind": "capsule", "metadata": {"labels": {"app": "web", "app1": "web1"}, "name": "template"}, "spec": {"containers": [{"workDir": "/root", "image": "ubuntu", "volumeMounts": [{"readOnly": true, "mountPath": "/data1", "name": "volume01"}], "command": ["/bin/bash"], "env": {"ENV2": "/usr/bin", "ENV1": "/usr/local/bin"}, "imagePullPolicy": "ifnotpresent", "ports": [{"containerPort": 80, "protocol": "TCP", "name": "nginx-port", "hostPort": 80}], "resources": {"requests": {"cpu": 1, "memory": 1024}}}, {"workDir": "/root", "image": "centos", "args": ["-c", "\"while true; do echo hello world; sleep 1; done\""], "volumeMounts": [{"mountPath": "/data2", "name": "volume02"}], "command": ["/bin/bash"], "env": {"ENV2": "/usr/bin/"}, "imagePullPolicy": "ifnotpresent", "ports": [{"containerPort": 80, "protocol": "TCP", "name": "nginx-port", "hostPort": 80}, {"containerPort": 3306, "protocol": "TCP", "name": "mysql-port", "hostPort": 3306}], "resources": {"requests": {"cpu": 1, "memory": 1024}}}], "volumes": [{"cinder": {"autoRemove": true, "size": 5}, "name": "volume01"}, {"cinder": {"autoRemove": true, "size": 5}, "name": "volume02"}]}, "restartPolicy": "Always", "capsuleVersion": "beta"}`)
        createOpts := capsules.CreateOpts{
                TemplateOpts:    template,
        }
	ret := capsules.Create(client, createOpts)
	server, id := ret.Extract()
	fmt.Println("====================")
	fmt.Println(id)
	fmt.Println(ret)
	fmt.Println("====================")
	fmt.Println(server)
}

