# Virt Network device plugin for Kubernetes
## Table of Contents

- [Virt Network device plugin](#virt-network-device-plugin)
- [Prerequisites](#prerequisites)
	-  [Supported virtual NICs](#supported-virtual-nics)
- [Quick Start](#quick-start)
	- [Network Object CRDs](#network-object-crds)
	- [Meta-Plugin CNI](#meta-plugin-cni) 
	 - [Enhanced host-device CNI](#enhanced-host-device-cni)
	 - [Build and run Virt Device plugin](#build-and-run-virt-device-plugin)
- [Issues and Contributing](#issues-and-contributing)

## Virt Network Device Plugin
The goal of the Virt Network device plugin is to simulate the lifecycle management of hardware network interfaces on a Kubernetes node.

- Device Plugin/Device Manager

  - Discovery of virtual NIC devices in a node

  - Advertisement of number of virtual interfaces available on a node

  - Allocation of virtual interface to a pod

- Meta plugin

  - Retrieve allocated DeviceIDs and associated resourceName from kubernetes APIServer.

  - Pass Device ID information to enhanced host-device CNI plugin

- Enhanced host-device CNI

  - On Cmd Add, plumbs allocated virtual interfaces to the pods network namespace using information passed from meta plugin

  - On Cmd Del, releases virtual interfaces from the pods network namespace

This implementation follows the directions of [this proposal document](https://docs.google.com/document/d/1Ewe9Of84GkP0b2Q2PC0y9RVZNkN2WeVEagX9m99Nrzc/).

## Prerequisites

There are list of items should be required before installing the Virt Network device plugin:

 1. Virtual interfaces - (Tested with virtio_net)

 2. Enhanced host-device CNI ([link](https://github.com/zshi-redhat/ehost-device-cni.git))

 3. Kubernetes version - 1.11+ (with [patch](https://github.com/kubernetes/kubernetes/compare/master...dashpole:device_id#diff-bf28da68f62a8df6e99e447c4351122))

 4. Meta plugin - Multus v3.0 (dev/k8s-deviceid-model branch)

Make sure to implement the steps described in [Quick Start](#quick-start) for Kubernetes cluster to support multi network.  Similar with SRIOV network device plugin, Virt network device plugin is a collective plugin model to work with device plugin, Meta-plugin and CNI plugin.

### Supported virtual NICs
The following virtual NIC was tested with this implementation.
-  virtio_net

## Quick Start
This section explains how to set up Virt Network device plugin in Kubernetes. Required YAML files can be found in [deployments/](deployments/) directory.

### Network Object CRDs

Kubernetes out of the box only allows to have one network interface per pod. In order to add multiple interfaces in a Pod we need to configure Kubernetes with a CNI meta plugin that enables invoking multiple CNI plugins to add additional interfaces.  [Multus](https://github.com/intel/multus-cni) is only meta plugin that supports this mechanism. Multus uses Kubernetes Custom Resource Definition or CRDs to define network objects. For more information see Multus [documentation](https://github.com/intel/multus-cni/blob/master/README.md). 

### Meta Plugin CNI

1. Compile Meta Plugin CNI (Multus dev/k8s-deviceid-model branch):
````
$ git clone https://github.com/intel/multus-cni.git
$ cd multus-cni
$ git fetch
$ git checkout dev/k8s-deviceid-model
$ ./build
$ cp bin/multus /opt/cni/bin
````

2. Configure Kubernetes network CRD with [Multus](https://github.com/intel/multus-cni/tree/dev/network-plumbing-working-group-crd-change#creating-network-resources-in-kubernetes)

### Enhanced host-device CNI

 Compile host-device CNI:

    $ git clone https://github.com/zshi-redhat/ehost-device-cni.git
    $ cd ehost-device-cni
    $ ./build
    $ cp bin/ehost-device /opt/cni/bin

### Build and run Virt Device plugin

 1. Clone the virt-network-device-plugin repository
 ```
$ git clone https://github.com/zshi-redhat/virt-network-device-plugin.git
 ```  

 2. Run the build script, this will build the Virt Network Device Plugin binary
 ``` 
$ ./build.sh
```      

 3. Create the virt Network CRD
```
$ kubectl create -f deployments/virt-crd.yaml
```
 
 4. Run build docker script to create Virt Network Device Plugin Docker image
 ```
$ cd deployments/
$ ./build_docker.sh
``` 
 5. Create Virt Network Device Plugin Pod
 ```
$ kubectl create -f pod-virtdp.yaml
```
 >Note: This is for demo purposes, the Virt Device Plugin binary must be executed from within the pod

 6. Get a bash terminal to the Virt Network Device Plugin Pod
 ```
$ kubectl exec -it virt-device-plugin bash
```

 7. Execute the Virt Network Device Plugin binary from within the Pod
````
$ ./usr/bin/virtdp --logtostderr -v 10

virt-device-plugin.go:284] Starting Virt Network Device Plugin...
virt-device-plugin.go:128] Discovering virtual network device[s]
virt-device-plugin.go:80] Checking for file /sys/class/net/cni0/device/driver 
virt-device-plugin.go:84] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/cni0/device/driver: no such file or directory
virt-device-plugin.go:80] Checking for file /sys/class/net/docker0/device/driver 
virt-device-plugin.go:84] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/docker0/device/driver: no such file or directory
virt-device-plugin.go:80] Checking for file /sys/class/net/eth0/device/driver 
virt-device-plugin.go:97] deviceInfo: ../../devices/pci0000:00/0000:00:03.0/virtio0/net/eth0
virt-device-plugin.go:80] Checking for file /sys/class/net/eth1/device/driver 
virt-device-plugin.go:97] deviceInfo: ../../devices/pci0000:00/0000:00:07.0/virtio3/net/eth1
virt-device-plugin.go:80] Checking for file /sys/class/net/lo/device/driver 
virt-device-plugin.go:84] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/lo/device/driver: no such file or directory
virt-device-plugin.go:133] Starting Virtual Network Device Plugin server at: /var/lib/kubelet/device-plugins/virtNet.sock
virt-device-plugin.go:157] Virt Network Device Plugin server started serving
virt-device-plugin.go:310] Virt Network Device Plugin registered with the Kubelet
virt-device-plugin.go:228] ListAndWatch: send devices &ListAndWatchResponse{Devices:[&Device{ID:0000:00:03.0,Health:Healthy,} &Device{ID:0000:00:07.0,Health:Healthy,}],}
````

## Issues and Contributing
We welcome your feedback and contributions to this project. Please see the [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines. 

Copyright 2018 © Red Hat Corporation.
