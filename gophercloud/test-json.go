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
	fmt.Printf("-------t.validate------------")
	fmt.Printf("%s", t)
	fmt.Printf("%s", t.Validate())
	fmt.Printf("-----------------------------")
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

func main() {
	templateJSON := new(Template)
	templateJSON.Bin = []byte(ValidJSONTemplate)
	templateJSON.Parse()
}

