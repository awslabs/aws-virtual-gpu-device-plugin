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

package nvidia

import (
	"fmt"
	"log"
	"strings"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"

	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

// Instead of returning physicial GPU devices, let's return vGPU devices here.
// Total number of vGPU depends on the phycial GPU memory / Memory Unit user specifies.
func getVGPUDevices(memoryUnit int) []*pluginapi.Device {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []*pluginapi.Device
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDevice(i)
		check(err)

		log.Printf("Device Memory: %d, vGPU Memory: %d", uint(*d.Memory), memoryUnit)
		vGPUCount := uint(*d.Memory) / uint(memoryUnit)

		for j := uint(0); j < vGPUCount; j++ {
			vGPUDeviceID := getVGPUID(d.UUID, j)
			dev := pluginapi.Device{
				ID:     vGPUDeviceID,
				Health: pluginapi.Healthy,
			}
			// if d.CPUAffinity != nil {
			// 	dev.Topology = &pluginapi.TopologyInfo{
			// 		Nodes: []*pluginapi.NUMANode{
			// 			&pluginapi.NUMANode{
			// 				ID: int64(*(d.CPUAffinity)),
			// 			},
			// 		},
			// 	}
			// }
			devs = append(devs, &dev)
		}
	}

	return devs
}

func GetDeviceCount() uint {
	n, err := nvml.GetDeviceCount()
	check(err)
	return n
}

func getPhysicalGPUDevices() []string {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []string
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDevice(i)
		check(err)
		devs = append(devs, d.UUID)
	}

	return devs
}

func getDevices() []*pluginapi.Device {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []*pluginapi.Device
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		check(err)

		dev := pluginapi.Device{
			ID:     d.UUID,
			Health: pluginapi.Healthy,
		}
		if d.CPUAffinity != nil {
			// dev.Topology = &pluginapi.TopologyInfo{
			// 	Nodes: []*pluginapi.NUMANode{
			// 		&pluginapi.NUMANode{
			// 			ID: int64(*(d.CPUAffinity)),
			// 		},
			// 	},
			// }
		}
		devs = append(devs, &dev)
	}

	return devs
}

func getVGPUID(deviceID string, vGPUIndex uint) string {
	return fmt.Sprintf("%s-%d", deviceID, vGPUIndex)
}

func getPhysicalDeviceID(vGPUDeviceID string) string {
	return strings.Split(vGPUDeviceID, "-")[0]
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}

func physicialDeviceExists(devs []string, id string) bool {
	for _, d := range devs {
		if d == id {
			return true
		}
	}
	return false
}

func watchXIDs(ctx context.Context, devs []*pluginapi.Device, xids chan<- *pluginapi.Device) {
	eventSet := nvml.NewEventSet()
	defer nvml.DeleteEventSet(eventSet)

	for _, d := range devs {
		err := nvml.RegisterEventForDevice(eventSet, nvml.XidCriticalError, d.ID)
		if err != nil && strings.HasSuffix(err.Error(), "Not Supported") {
			log.Printf("Warning: %s is too old to support healthchecking: %s. Marking it unhealthy.", d.ID, err)

			xids <- d
			continue
		}

		if err != nil {
			log.Panicln("Fatal:", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		e, err := nvml.WaitForEvent(eventSet, 5000)
		if err != nil && e.Etype != nvml.XidCriticalError {
			continue
		}

		// FIXME: formalize the full list and document it.
		// http://docs.nvidia.com/deploy/xid-errors/index.html#topic_4
		// Application errors: the GPU should still be healthy
		if e.Edata == 31 || e.Edata == 43 || e.Edata == 45 {
			continue
		}

		if e.UUID == nil || len(*e.UUID) == 0 {
			// All devices are unhealthy
			for _, d := range devs {
				log.Printf("XidCriticalError: Xid=%d, All devices will go unhealthy.", e.Edata)
				xids <- d
			}
			continue
		}

		for _, d := range devs {
			if d.ID == *e.UUID {
				log.Printf("XidCriticalError: Xid=%d on GPU=%s, the device will go unhealthy.", e.Edata, d.ID)
				xids <- d
			}
		}
	}
}
