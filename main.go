/*
 * Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"errors"
	"flag"

	"log"
	"os"
	"syscall"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/aws/eks-virtual-gpu/pkg/gpu/nvidia"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var (
	memoryPerVirtualGPU = flag.Int("memory-per-virtual-gpu", 1024, "Set GPU Memory for virtual GPU, support 'MiB'")
	mps                 = flag.Bool("mps", false, "Enable or Disable MPS")
	healthCheck         = flag.Bool("health-check", false, "Enable or disable Health check")
	// TODO: we could ask user to pass GPU count they'd like to virtualize
)

func main() {
	// Add parameter support?
	// Receive  --vGPU limit -> has to be divided by 16GiB.
	// 1. Maintain the target vGPU we want to advertise
	// 2. ListAndWatch -> report vGPU
	// 3. Allocate -> do binpacking, since it's just one GPU, it makes sense.

	flag.Parse()
	log.Println("Start Amazon EKS vGPU device plugin")

	run(*mps, *healthCheck, *memoryPerVirtualGPU)
}

func run(enableMPS, enableHealthCheck bool, memoryUnit int) {
	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.Printf("Failed to initialize NVML: %s.", err)
		log.Printf("If this is a GPU node, did you set the docker default runtime to `nvidia`?")

		// TODO: point to our repo
		log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")

		select {}
	}
	defer func() { log.Println("Shutdown of NVML returned:", nvml.Shutdown()) }()

	// Check if MemoryUnit is a valid value.
	if err := validMemoryUnit(memoryUnit); err != nil {
		log.Printf("Failed to valid memoryUnit: %s.", err)
		os.Exit(1)
	}

	log.Println("Fetching devices.")
	if len(getDeviceCount()) == 0 {
		log.Println("No devices found. Waiting indefinitely.")
		select {}
	}

	log.Println("Starting FS watcher.")
	watcher, err := nvidia.NewFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := nvidia.NewOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *nvidia.NvidiaDevicePlugin

L:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}

			devicePlugin = nvidia.NewNvidiaDevicePlugin(enableMPS, enableHealthCheck, memoryUnit)
			if err := devicePlugin.Serve(); err != nil {
				log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
				// TODO: point to our own docs.
				log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
				log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				restart = true
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				devicePlugin.Stop()
				break L
			}
		}
	}

}

// retrieve one GPU and check memory / MemoryUnit is equals 0 or not.
func validMemoryUnit(memoryUnit int) error {
	n, err := nvml.GetDeviceCount()
	check(err)

	// AWS EC2 has exact same GPUs on single instance so it's safe to pick any one for validation
	d, err := nvml.NewDevice(0)
	check(err)

	if uint(*d.Memory)%memoryUnit != 0 {
		return errors.New("Current GPU Model %s has total memory %d which can not divided by MemoryUnit %d", *d.Model, *d.Memory, memoryUnit)
	}
}

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}
