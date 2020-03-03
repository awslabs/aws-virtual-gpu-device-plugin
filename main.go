package main

import (
	"flag"
	"log"

	"github.com/awslabs/aws-virtual-gpu-device-plugin/pkg/gpu/nvidia"
)

var (
	vGPU = flag.Int("vgpu", 10, "Number of virtual GPUs")
)

const VOLTA_MAXIMUM_MPS_CLIENT = 48

func main() {
	flag.Parse()
	log.Println("Start virtual GPU device plugin")

	if *vGPU > VOLTA_MAXIMUM_MPS_CLIENT {
		log.Fatal("Number of virtual GPUs can not exceed maximum number of MPS clients")
	}

	vgm := nvidia.NewVirtualGPUManager(*vGPU)

	err := vgm.Run()
	if err != nil {
		log.Fatalf("Failed due to %v", err)
	}
}
