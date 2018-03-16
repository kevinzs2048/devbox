package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
        "github.com/gophercloud/gophercloud"
)

// TE is a base structure for both Template and Environment
type TE struct {
	// Bin stores the contents of the template or environment.
	Bin []byte
	// Parsed contains a parsed version of Bin. Since there are 2 different
	// fields referring to the same value, you must be careful when accessing
	// this filed.
	Parsed map[string]interface{}
}

// Template is a structure that represents OpenStack Heat templates
type Template struct {
	TE
}

type ErrInvalidDataFormat struct {
	gophercloud.BaseError
}

// Parse will parse the contents and then validate. The contents MUST be either JSON or YAML.
func (t *TE) Parse() {
	if jerr := json.Unmarshal(t.Bin, &t.Parsed); jerr != nil {
		fmt.Printf("========jjjerr===========")
		fmt.Printf("%s", jerr)
		fmt.Printf("===================")
		if yerr := yaml.Unmarshal(t.Bin, &t.Parsed); yerr != nil {
			fmt.Printf("========yyyerr===========")
			fmt.Printf("%s", yerr)
			fmt.Printf("===================")
		}
	}
	fmt.Printf("\r\n-------t.validate------------\r\n")
	fmt.Printf("%s", t)
	fmt.Printf("%s", t.Validate())
	fmt.Printf("\r\n--------Mission finished------------\r\n")
}

func (t *TE) ParseNo() {
	if jerr := json.Unmarshal(t.Bin, &t.Parsed); jerr != nil {
		if yerr := yaml.Unmarshal(t.Bin, &t.Parsed); yerr != nil {
			fmt.Printf("%s", yerr)
		}
	}
}
// Validate validates the contents of TE
func (t *TE) Validate() error {
	return nil
}

// ValidJSONTemplate is a valid OpenStack Heat template in JSON format
const ValidJSONTemplate = `
{
    "capsuleVersion": "beta",
    "kind": "capsule",
    "metadata": {
        "labels": {
            "app": "web",
            "app1": "web1"
        },
        "name": "template"
    },
    "restartPolicy": "Always",
    "spec": {
        "containers": [
            {
                "command": [
                    "/bin/bash"
                ],
                "env": {
                    "ENV1": "/usr/local/bin"
                },
                "image": "ubuntu",
                "imagePullPolicy": "ifnotpresent",
                "ports": [
                    {
                        "containerPort": 80,
                        "hostPort": 80,
                        "name": "nginx-port",
                        "protocol": "TCP"
                    }
                ],
                "resources": {
                    "requests": {
                        "cpu": 1,
                        "memory": 1024
                    }
                },
                "volumeMounts": [
                    {
                        "mountPath": "/data1",
                        "name": "volume01",
                        "readOnly": true
                    }
                ],
                "workDir": "/root"
            }
        ],
        "volumes": [
            {
                "cinder": {
                    "autoRemove": true,
                    "size": 5
                },
                "name": "volume01"
            }
        ]
    }
}
`

// ValidYAMLTemplate is a valid OpenStack Heat template in YAML format
const ValidYAMLTemplate = `
capsuleVersion: beta
kind: capsule
metadata:
  name: template
  labels:
    app: web
    app1: web1
restartPolicy: Always
spec:
  containers:
  - image: ubuntu
    command:
      - "/bin/bash"
    imagePullPolicy: ifnotpresent
    workDir: /root
    ports:
      - name: nginx-port
        containerPort: 80
        hostPort: 80
        protocol: TCP
    resources:
      requests:
        cpu: 1
        memory: 1024
    env:
      ENV1: /usr/local/bin
      ENV2: /usr/bin
`

func main() {
	templateJSON := new(Template)
	templateJSON.Bin = []byte(ValidJSONTemplate)
	templateJSON.ParseNo()
	fmt.Printf("%s", templateJSON.Parsed)
        fmt.Printf("\r\n-----------------------------------\r\n")

        templateJSON = new(Template)
	templateJSON.Bin = []byte(ValidYAMLTemplate)
	templateJSON.Parse()
	fmt.Printf("%s", templateJSON.Parsed)
}

