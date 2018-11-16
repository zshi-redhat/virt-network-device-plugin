// Copyright 2018 Intel Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubernetes/pkg/kubelet/apis/pluginregistration/v1beta1"
)

const (
	netDirectory    = "/sys/class/net/"
	routePath       = "/proc/net/route"

	// Device plugin settings.
	pluginMountPath      = "/var/lib/kubelet/plugins"
	pluginEndpointPrefix = "virtNet"
	resourceName         = "kernel.org/virt"
)

// virtManager manages virtual networking devices
type virtManager struct {
	socketFile       string
	devices          map[string]pluginapi.Device   // for Kubelet DP API
	grpcServer       *grpc.Server
}

func newVirtManager() *virtManager {

	return &virtManager{
		devices:          make(map[string]pluginapi.Device),
		socketFile:       fmt.Sprintf("%s.sock", pluginEndpointPrefix),
	}
}

// Returns a list of virtual interface names as string
func getVirtualInterfaceList() (map[string]string, error) {

	virtNetDevices := make(map[string]string)
	var defaultInterface string

	netDevices, err := ioutil.ReadDir(netDirectory)
	if err != nil {
		glog.Errorf("Error. Cannot read %s for network device names. Err: %v", netDirectory, err)
		return virtNetDevices, err
	}

	if len(netDevices) < 1 {
		glog.Errorf("Error. No network device found in %s directory", netDirectory)
		return virtNetDevices, err
	}

	routeFile, err := os.Open(routePath)
	if err != nil {
		glog.Errorf("Error. Cannot read %s for default route interface. Err: %v", routePath, err)
		return virtNetDevices, err
	}
	defer routeFile.Close()

	scanner := bufio.NewScanner(routeFile)
	for scanner.Scan() {
		scanner.Scan()
		defaultInterface = strings.Split(scanner.Text(), "\t")[0]
		break
	}

	for _, dev := range netDevices {

		if dev.Name() == defaultInterface {
			glog.Infof("Skipping default interface %s ", defaultInterface)
			continue
		}

		driverPath := filepath.Join(netDirectory, dev.Name(), "device", "driver")
		glog.Infof("Checking for file %s ", driverPath)

		driverInfo, err := os.Readlink(driverPath)
		if err != nil {
			glog.Infof("Cannot read driver symbolic link - Skipping: %v", err)
			continue
		}

		driver := driverInfo[len("../../../../bus/virtio/drivers/"):]

		if driver == "virtio_net" {
			devicePath := filepath.Join(netDirectory, dev.Name())
			deviceInfo, err := os.Readlink(devicePath)
			if err != nil {
				glog.Infof("Cannot read device symbolic link - Skipping: %v", err)
				continue
			}
			glog.Infof("deviceInfo: %s", deviceInfo)
			pciStr := deviceInfo[len("../../devices/pci0000:00/"):]
			pciAddr := strings.Split(pciStr, "/")
			virtNetDevices[dev.Name()] = pciAddr[0]
		}
	}

	return virtNetDevices, nil
}

//Reads DeviceName and gets PCI Addresses of virtual interfaces
func (vm *virtManager) discoverNetworks() error {

	var healthValue string
	virtMap, err := getVirtualInterfaceList()
	if err != nil {
		glog.Errorf("Error. No Virtual network device found")
		return err
	}
	for name, addr := range virtMap {
		if IsNetlinkStatusUp(name) {
			healthValue = pluginapi.Healthy
		} else {
			healthValue = "Unhealthy"
		}
		vm.devices[addr] = pluginapi.Device{ID: addr, Health: healthValue}
	}
	return nil
}

// IsNetlinkStatusUp returns 'false' if 'operstate' is not "up" for a Linux netowrk device
func IsNetlinkStatusUp(dev string) bool {
	opsFile := filepath.Join(netDirectory, dev, "operstate")
	bytes, err := ioutil.ReadFile(opsFile)
	if err != nil || strings.TrimSpace(string(bytes)) != "up" {
		return false
	}
	return true
}

// Probe returns 'true' if device changes detected 'false' otherwise
func (vm *virtManager) Probe() bool {

// TODO Probe link state of allocated device in another network namespace
/*
	var healthValue string
	currentDevices := make(map[string]pluginapi.Device)

	virtMap, err := getVirtualInterfaceList()
	if err != nil {
		glog.Errorf("Error. No Virtual network device found")
		return false
	}
	for name, addr := range virtMap {
		if IsNetlinkStatusUp(name) {
			healthValue = pluginapi.Healthy
		} else {
			healthValue = "Unhealthy"
		}
		currentDevices[addr] = pluginapi.Device{ID: addr, Health: healthValue}
	}
	if !reflect.DeepEqual(vm.devices, currentDevices) {
		vm.devices = currentDevices
		return true
	}
*/
	return false
}

// Discovers capabable virtual devices
func (vm *virtManager) Start() error {
	pluginEndpoint := filepath.Join(pluginMountPath, vm.socketFile)
	glog.Infof("Starting Virtual Network Device Plugin server at: %s\n", pluginEndpoint)
	lis, err := net.Listen("unix", pluginEndpoint)
	if err != nil {
		glog.Errorf("Error. Starting Virtual Network Device Plugin server failed: %v", err)
	}
	vm.grpcServer = grpc.NewServer()

	// Register virt device plugin service
	registerapi.RegisterRegistrationServer(vm.grpcServer, vm)
	pluginapi.RegisterDevicePluginServer(vm.grpcServer, vm)

	go vm.grpcServer.Serve(lis)

	// Wait for server to start by launching a blocking connection
	conn, err := grpc.Dial(pluginEndpoint, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		glog.Errorf("Error. Could not establish connection with gRPC server: %v", err)
		return err
	}
	glog.Infoln("Virt Network Device Plugin server started serving")
	conn.Close()
	return nil
}

func (vm *virtManager) Stop() error {
	glog.Infof("Stopping Virt Network Device Plugin gRPC server..")
	if vm.grpcServer == nil {
		return nil
	}

	vm.grpcServer.Stop()
	vm.grpcServer = nil

	return vm.cleanup()
}

// Removes existing socket if exists
// [adpoted from https://github.com/redhat-nfvpe/k8s-dummy-device-plugin/blob/master/dummy.go ]
func (vm *virtManager) cleanup() error {
	pluginEndpoint := filepath.Join(pluginMountPath, vm.socketFile)
	if err := os.Remove(pluginEndpoint); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (vm *virtManager) GetInfo(ctx context.Context, rqt *registerapi.InfoRequest) (*registerapi.PluginInfo, error) {
	return &registerapi.PluginInfo{Type: registerapi.DevicePlugin, Name: resourceName, Endpoint: filepath.Join(pluginMountPath, vm.socketFile), SupportedVersions: []string{"v1beta1"}}, nil
}

func (vm *virtManager) NotifyRegistrationStatus(ctx context.Context, regstat *registerapi.RegistrationStatus) (*registerapi.RegistrationStatusResponse, error) {
	out := new(registerapi.RegistrationStatusResponse)
	if regstat.PluginRegistered {
		glog.Infof("Plugin: %s gets registered successfully at Kubelet\n", vm.socketFile)
	} else {
		glog.Infof("Plugin:%s failed to registered at Kubelet: %v; shutting down.\n", vm.socketFile, regstat.Error)
		vm.Stop()
	}
	return out, nil
}

// Implements DevicePlugin service functions
func (vm *virtManager) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	resp := new(pluginapi.ListAndWatchResponse)
	for _, dev := range vm.devices {
		resp.Devices = append(resp.Devices, &pluginapi.Device{ID: dev.ID, Health: dev.Health})
	}
	glog.Infof("ListAndWatch: send initial devices %v\n", resp)
	if err := stream.Send(resp); err != nil {
		glog.Errorf("Error. Cannot update initial device states: %v\n", err)
		vm.grpcServer.Stop()
		return err
	}

	for {
		select {
		case <-time.After(10 * time.Second):
		}

		if vm.Probe() {
			resp := new(pluginapi.ListAndWatchResponse)
			for _, dev := range vm.devices {
				resp.Devices = append(resp.Devices, &pluginapi.Device{ID: dev.ID, Health: dev.Health})
			}
			glog.Infof("ListAndWatch: send devices %v\n", resp)
			if err := stream.Send(resp); err != nil {
				glog.Errorf("Error. Cannot update device states: %v\n", err)
				vm.grpcServer.Stop()
				return err
			}
		}
	}
	return nil
}

func (vm *virtManager) PreStartContainer(ctx context.Context, psRqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (vm *virtManager) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

//Allocate passes the PCI Addr(s) as an env variable to the requesting container
func (vm *virtManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp := new(pluginapi.AllocateResponse)
	pciAddrs := ""
	for _, container := range rqt.ContainerRequests {
		containerResp := new(pluginapi.ContainerAllocateResponse)
		for _, id := range container.DevicesIDs {
			glog.Infof("DeviceID in Allocate: %v", id)
			dev, ok := vm.devices[id]
			if !ok {
				glog.Errorf("Error. Invalid allocation request with non-existing device %s", id)
				return nil, fmt.Errorf("Error. Invalid allocation request with non-existing device %s", id)
			}
			if dev.Health != pluginapi.Healthy {
				glog.Errorf("Error. Invalid allocation request with unhealthy device %s", id)
				return nil, fmt.Errorf("Error. Invalid allocation request with unhealthy device %s", id)
			}

			pciAddrs = pciAddrs + id + ","
		}

		glog.Infof("PCI Addrs allocated: %s", pciAddrs)
		envmap := make(map[string]string)
		envmap["VIRT-PCI-ADDR"] = pciAddrs

		containerResp.Envs = envmap
		resp.ContainerResponses = append(resp.ContainerResponses, containerResp)
	}
	return resp, nil
}

func main() {
	flag.Parse()
	defer glog.Flush()
	glog.Infof("Starting Virt Network Device Plugin...")
	vm := newVirtManager()
	if vm == nil {
		glog.Errorf("Unable to get instance of a Virt-Manager")
		return
	}
	vm.cleanup()

	// respond to syscalls for termination
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Discover VIRT network device(s)
	if err := vm.discoverNetworks(); err != nil {
		glog.Errorf("virtManager.discoverNetworks() failed: %v", err)
		return
	}
	// Start server
	if err := vm.Start(); err != nil {
		glog.Errorf("virtManager.Start() failed: %v", err)
		return
	}

	// Catch termination signals
	select {
	case sig := <-sigCh:
		glog.Infof("Received signal \"%v\", shutting down.", sig)
		vm.Stop()
		return
	}
}
