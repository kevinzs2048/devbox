#!/bin/bash

source ~/devstack/openrc admin admin

zun capsule-delete capsule-demo

zun capsule-create -f demo.yaml
