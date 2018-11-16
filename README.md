# Virt Network device plugin for Kubernetes
[![Travis CI](https://travis-ci.org/zshi-redhat/virt-network-device-plugin.svg?branch=master)](https://travis-ci.org/zshi-redhat/virt-network-device-plugin/builds)

## Table of Contents

- [Virt Network device plugin](#virt-network-device-plugin)
- [Prerequisites](#prerequisites)
	- [Supported virtual NICs](#supported-virtual-nics)
- [Quick Start](#quick-start)
	- [Network Object CRDs](#network-object-crds)
	- [Meta-Plugin CNI](#meta-plugin-cni) 
	- [Enhanced host-device CNI](#enhanced-host-device-cni)
	- [Build and run Virt Device plugin](#build-and-run-virt-device-plugin)
	- [Testing](#testing)
- [Issues and Contributing](#issues-and-contributing)

## Virt Network Device Plugin
The goal of the Virt Network device plugin is to simulate the lifecycle management of hardware network interfaces on a Kubernetes node.

- Device Plugin/Device Manager

  - Discovery of virtual NIC devices in a node

  - Advertisement of number of virtual interfaces available on a node

  - Allocation of virtual interface to a pod

- Meta CNI plugin (e.g Multus)

  - Retrieve allocated DeviceIDs and associated resourceName from kubernetes APIServer.

  - Pass Device ID information to enhanced host-device CNI plugin

- Enhanced host-device CNI

  - On Cmd Add, plumbs allocated virtual interfaces to the pods network namespace using information passed from meta plugin

  - On Cmd Del, releases virtual interfaces from the pods network namespace

This implementation follows the directions of [this proposal document](https://docs.google.com/document/d/1Ewe9Of84GkP0b2Q2PC0y9RVZNkN2WeVEagX9m99Nrzc/).

## Prerequisites

There are list of items should be required before installing the Virt Network device plugin:

 1. Virtual interfaces - (Virt-network-device-plugin discovers devices with virtio_net driver type as available hardware resource, it assumes virtio_net interface has already been created in the VM where virt-network-device-plugin is running)

 2. Enhanced host-device CNI ([link](https://github.com/zshi-redhat/ehost-device-cni.git))

 3. Kubernetes version - 1.10+

 4. Meta plugin - Multus (master branch)

Make sure to implement the steps described in [Quick Start](#quick-start) for Kubernetes cluster to support multi network.  Similar with SRIOV network device plugin, Virt network device plugin is a collective plugin model to work with CNI plugins.

### Supported virtual NICs
The following virtual NIC was tested with this implementation.
-  virtio_net

## Quick Start
This section explains how to set up Virt Network device plugin in Kubernetes. Required YAML files can be found in [deployments/](deployments/) directory.

### Network Object CRDs

Kubernetes out of the box only allows to have one network interface per pod. In order to add multiple interfaces in a Pod we need to configure Kubernetes with a CNI meta plugin that enables invoking multiple CNI plugins to add additional interfaces.  [Multus](https://github.com/intel/multus-cni) is only meta plugin that supports this mechanism. Multus uses Kubernetes Custom Resource Definition or CRDs to define network objects. For more information see Multus [documentation](https://github.com/intel/multus-cni/blob/master/README.md). 

### Meta Plugin CNI

1. Compile Meta Plugin CNI (Multus master branch):
````
$ git clone https://github.com/intel/multus-cni.git
$ cd multus-cni
$ ./build
$ cp bin/multus /opt/cni/bin
````

### Enhanced host-device CNI

1. Compile host-device CNI plugin:
```
    $ git clone https://github.com/zshi-redhat/ehost-device-cni.git
    $ cd ehost-device-cni
    $ ./build
    $ cp bin/ehost-device /opt/cni/bin
```

### Build and run Virt Device plugin

 1. Clone the virt-network-device-plugin repository
 ```
$ git clone https://github.com/zshi-redhat/virt-network-device-plugin.git
 ```  

 2. Run the build script, this will build the Virt Network Device Plugin binary
 ``` 
$ cd virt-network-device-plugin/
$ ./build.sh
```      

 3. Run build docker script to create Virt Network Device Plugin Docker image
 ```
$ cd deployments/
$ ./build_docker.sh

``` 

 4. Create the virt Network CRD
```
$ kubectl create -f crdnetwork.yaml
$ kubectl create -f virt-crd.yaml
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

I0907 06:04:47.345638   14553 virt-device-plugin.go:307] Starting Virt Network Device Plugin...
I0907 06:04:47.347127   14553 virt-device-plugin.go:151] Discovering virtual network device[s]
I0907 06:04:47.348094   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/cni0/device/driver 
I0907 06:04:47.348172   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/cni0/device/driver: no such file or directory
I0907 06:04:47.348265   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/dev18303/device/driver 
I0907 06:04:47.348385   14553 virt-device-plugin.go:120] deviceInfo: ../../devices/pci0000:00/0000:00:0a.0/virtio6/net/dev18303
I0907 06:04:47.348421   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/docker0/device/driver 
I0907 06:04:47.348476   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/docker0/device/driver: no such file or directory
I0907 06:04:47.348499   14553 virt-device-plugin.go:98] Skipping default interface eth0 
I0907 06:04:47.348517   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/lo/device/driver 
I0907 06:04:47.348567   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/lo/device/driver: no such file or directory
I0907 06:04:47.348589   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/net1/device/driver 
I0907 06:04:47.348670   14553 virt-device-plugin.go:120] deviceInfo: ../../devices/pci0000:00/0000:00:09.0/virtio5/net/net1
I0907 06:04:47.348691   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/veth387624d9/device/driver 
I0907 06:04:47.348997   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/veth387624d9/device/driver: no such file or directory
I0907 06:04:47.349039   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/veth57c41295/device/driver 
I0907 06:04:47.349092   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/veth57c41295/device/driver: no such file or directory
I0907 06:04:47.349115   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/veth8a520c05/device/driver 
I0907 06:04:47.349165   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/veth8a520c05/device/driver: no such file or directory
I0907 06:04:47.349187   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/veth9d9e3ebe/device/driver 
I0907 06:04:47.349236   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/veth9d9e3ebe/device/driver: no such file or directory
I0907 06:04:47.349258   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/vetha0f18170/device/driver 
I0907 06:04:47.349307   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/vetha0f18170/device/driver: no such file or directory
I0907 06:04:47.349343   14553 virt-device-plugin.go:103] Checking for file /sys/class/net/vethcd80aa2f/device/driver 
I0907 06:04:47.349397   14553 virt-device-plugin.go:107] Cannot read driver symbolic link - Skipping: readlink /sys/class/net/vethcd80aa2f/device/driver: no such file or directory
I0907 06:04:47.349465   14553 virt-device-plugin.go:156] Starting Virtual Network Device Plugin server at: /var/lib/kubelet/device-plugins/virtNet.sock
I0907 06:04:47.353794   14553 virt-device-plugin.go:180] Virt Network Device Plugin server started serving
I0907 06:04:47.363086   14553 virt-device-plugin.go:333] Virt Network Device Plugin registered with the Kubelet
I0907 06:04:47.364213   14553 virt-device-plugin.go:251] ListAndWatch: send devices &ListAndWatchResponse{Devices:[&Device{ID:0000:00:0a.0,Health:Healthy,} &Device{ID:0000:00:09.0,Health:Healthy,}],}

````

### Testing

Leave the virt network device plugin running and open a new terminal session for following steps.

 1. Deploy test Pod
 ```
$ kubectl create -f pod-tc1.yaml
pod/testpod1 created
```

 2. Check Pod status
 ```
$ kubectl get pods
NAME                 READY     STATUS    RESTARTS   AGE
testpod1             1/1       Running   0          5s
virt-device-plugin   1/1       Running   0          16m
```

 3. Check virt-network-device-plugin log, new DeviceID be allocated to testpod1
 ```
I0907 06:21:09.234115   14553 virt-device-plugin.go:281] DeviceID in Allocate: 0000:00:0a.0
I0907 06:21:09.234613   14553 virt-device-plugin.go:295] PCI Addrs allocated: 0000:00:0a.0,
```

 4. Verify network interface be attached to Pod
 ```
$ kubectl exec -it testpod1 -- ip addr show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
3: eth0@if19642: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UP group default 
    link/ether 0a:58:0a:60:4c:34 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.96.76.52/16 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::8c7:64ff:fe19:cab4/64 scope link 
       valid_lft forever preferred_lft forever
18303: net1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 52:54:00:c4:e6:cd brd ff:ff:ff:ff:ff:ff
    inet 10.56.218.17/16 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:fec4:e6cd/64 scope link 
       valid_lft forever preferred_lft forever
```

 5. Verify network interface PCI address is equal to Device ID allocated by virt-network-device-plugin
 ```
$ kubectl exec -it testpod1 -- ethtool -i net1
driver: virtio_net
version: 1.0.0
firmware-version: 
expansion-rom-version: 
bus-info: 0000:00:0a.0
supports-statistics: no
supports-test: no
supports-eeprom-access: no
supports-register-dump: no
supports-priv-flags: no
```

  6. Create another Pod and test traffic between two Pods
 ```
$ kubectl create -f pod-tc2.yaml
pod/testpod2 created

$ kubectl get pods
NAME                 READY     STATUS    RESTARTS   AGE
testpod1             1/1       Running   0          7m
testpod2             1/1       Running   0          5s
virt-device-plugin   1/1       Running   0          24m

$ kubectl exec -it testpod2 -- ip addr show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
3: eth0@if19643: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UP group default 
    link/ether 0a:58:0a:60:4c:35 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.96.76.53/16 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::2c00:1aff:fe8a:261e/64 scope link 
       valid_lft forever preferred_lft forever
18302: net1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 52:54:00:2e:24:f7 brd ff:ff:ff:ff:ff:ff
    inet 10.56.218.18/16 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:fe2e:24f7/64 scope link 
       valid_lft forever preferred_lft forever

$ kubectl exec -it testpod2 -- ping -c2 10.56.218.17
PING 10.56.218.17 (10.56.218.17) 56(84) bytes of data.
64 bytes from 10.56.218.17: icmp_seq=1 ttl=64 time=0.712 ms
64 bytes from 10.56.218.17: icmp_seq=2 ttl=64 time=0.574 ms

--- 10.56.218.17 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1000ms
rtt min/avg/max/mdev = 0.574/0.643/0.712/0.069 ms
```

 >Note: 'net1' interface in each pod is the device allocated from virt-network-device-plugin; '10.56.218.17' is IP address of net1 in testpod1, '10.56.218.18' is IP address of net1 in testpod2

## Issues and Contributing
We welcome your feedback and contributions to this project. Please see the [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines. 

Copyright 2018 © Red Hat Corporation.
