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
	"flag"

	"log"

	"github.com/aws/eks-virtual-gpu/pkg/gpu/nvidia"
)

var (
	mps                 = flag.Bool("mps", false, "Enable or Disable MPS")
	healthCheck         = flag.Bool("health-check", false, "Enable or disable Health check")
	memoryPerVirtualGPU = flag.Int("memory-per-virtual-gpu", 1024, "Set GPU Memory for virtual GPU, support 'MiB'")
)

func main() {
	flag.Parse()
	log.Println("Start Amazon EKS vGPU device plugin")

	vgm := nvidia.NewVirtualGPUManager(*mps, *healthCheck, *memoryPerVirtualGPU)

	err := vgm.Run()
	if err != nil {
		log.Fatalf("Failed due to %v", err)
	}
}
