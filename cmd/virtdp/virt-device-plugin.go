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
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	netDirectory    = "/sys/class/net/"

	// Device plugin settings.
	pluginMountPath      = "/var/lib/kubelet/device-plugins"
	kubeletEndpoint      = "kubelet.sock"
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
func getVirtualInterfaceList() ([]string, error) {

	virtNetDevices := []string{}

	netDevices, err := ioutil.ReadDir(netDirectory)
	if err != nil {
		glog.Errorf("Error. Cannot read %s for network device names. Err: %v", netDirectory, err)
		return virtNetDevices, err
	}

	if len(netDevices) < 1 {
		glog.Errorf("Error. No network device found in %s directory", netDirectory)
		return virtNetDevices, err
	}

	for _, dev := range netDevices {
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
			virtNetDevices = append(virtNetDevices, pciAddr[0])
		}
	}

	return virtNetDevices, nil
}

//Reads DeviceName and gets PCI Addresses of virtual interfaces
func (vm *virtManager) discoverNetworks() error {

	virtList, err := getVirtualInterfaceList()
	if err != nil {
		glog.Errorf("Error. No Virtual network device found")
		return err
	}
	for _, dev := range virtList {
		vm.devices[dev] = pluginapi.Device{ID: dev, Health: pluginapi.Healthy}
	}
	return nil
}

func (vm *virtManager) GetDeviceState(DeviceName string) string {
	// TODO: Discover device health
	return pluginapi.Healthy
}

// Discovers capabable virtual devices
func (vm *virtManager) Start() error {
	glog.Infof("Discovering virtual network device[s]")
	if err := vm.discoverNetworks(); err != nil {
		return err
	}
	pluginEndpoint := filepath.Join(pluginapi.DevicePluginPath, vm.socketFile)
	glog.Infof("Starting Virtual Network Device Plugin server at: %s\n", pluginEndpoint)
	lis, err := net.Listen("unix", pluginEndpoint)
	if err != nil {
		glog.Errorf("Error. Starting Virtual Network Device Plugin server failed: %v", err)
	}
	vm.grpcServer = grpc.NewServer()

	// Register virt device plugin service
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
	pluginEndpoint := filepath.Join(pluginapi.DevicePluginPath, vm.socketFile)
	if err := os.Remove(pluginEndpoint); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Register registers as a grpc client with the kubelet.
func Register(kubeletEndpoint, pluginEndpoint, resourceName string) error {
	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		glog.Errorf("Virt Network Device Plugin cannot connect to Kubelet service: %v", err)
		return err
	}
	defer conn.Close()
	client := pluginapi.NewRegistrationClient(conn)

	request := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndpoint,
		ResourceName: resourceName,
	}

	if _, err = client.Register(context.Background(), request); err != nil {
		glog.Errorf("Virt Network Device Plugin cannot register to Kubelet service: %v", err)
		return err
	}
	return nil
}

// Implements DevicePlugin service functions
func (vm *virtManager) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	changed := true
	for {
		for id, dev := range vm.devices {
			state := vm.GetDeviceState(id)
			if dev.Health != state {
				changed = true
				dev.Health = state
				vm.devices[id] = dev
			}
		}
		if changed {
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
		changed = false
		time.Sleep(5 * time.Second)
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

	// Start server
	if err := vm.Start(); err != nil {
		glog.Errorf("virtManager.Start() failed: %v", err)
		return
	}

	// Registers with Kubelet.
	err := Register(path.Join(pluginMountPath, kubeletEndpoint), vm.socketFile, resourceName)
	if err != nil {
		// Stop server
		vm.grpcServer.Stop()
		glog.Fatal(err)
		return
	}
	glog.Infof("Virt Network Device Plugin registered with the Kubelet")

	// Catch termination signals
	select {
	case sig := <-sigCh:
		glog.Infof("Received signal \"%v\", shutting down.", sig)
		vm.Stop()
		return
	}
}
