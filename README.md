# virtual GPU device plugin for Kubernetes

The virtual device plugin for Kubernetes is a Daemonset that allows you to automatically:
- Expose arbitrary number of virtual GPUs on GPU nodes of your cluster.
- Run ML serving containers backed by Accelerator with low latency and low cost in your Kubernetes cluster.

This repository contains NVIDIA's official implementation of the [Kubernetes device plugin](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md).

## Prerequisites

The list of prerequisites for running the virtual device plugin is described below:
* NVIDIA drivers ~= 361.93
* nvidia-docker version > 2.0 (see how to [install](https://github.com/NVIDIA/nvidia-docker) and it's [prerequisites](https://github.com/nvidia/nvidia-docker/wiki/Installation-\(version-2.0\)#prerequisites))
* docker configured with nvidia as the [default runtime](https://github.com/NVIDIA/nvidia-docker/wiki/Advanced-topics#default-runtime).
* Kubernetes version >= 1.10

## Limitations

* This solution is build on top of Volta [Multi-Process Service(MPS)](https://docs.nvidia.com/deploy/pdf/CUDA_Multi_Process_Service_Overview.pdf). You can only use it on instances types with Tesla-V100 or newer. (Only P3 now) 
* Virtual GPU device plugin by default set GPU compute mode to `EXCLUSIVE_PROCESS` which means GPU is assigned to MPS process, individual process threads can submit work to GPU concurrently via MPS server. 
This GPU can not be used for other purpose.
* Virtual GPU device plugin only on single physical GPU instance like P3.2xlarge if you request `eks.amazonaws.com/vgpu` more than 1 in the workloads.
* Virtual GPU device plugin can not work with [Nvidia device plugin](https://github.com/NVIDIA/k8s-device-plugin) together. You can label nodes and use selector to install Virtual GPU device plugin.

## Quick Start

### Label GPU node groups

```bash
kubectl label node ip-192-168-95-95.us-west-2.compute.internal k8s.amazonaws.com/accelerator=vgpu
```

### Enabling virtual GPU Support in Kubernetes

Update node selector label in the manifest file to match with labels of your GPU node group, then apply it to Kubernetes.

```shell
$ kubectl create -f https://raw.githubusercontent.com/aws/eks-virtual-gpu/0.1.0/nvidia-device-plugin.yml
```

### Running GPU Jobs

NVIDIA GPUs can now be consumed via container level resource requirements using the resource name nvidia.com/gpu:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: resnet-deployment
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: resnet-server
    spec:
      hostIPC: true
      containers:
      - name: resnet-container
        image: seedjeffwan/tensorflow-serving-gpu:resnet
        args:
        # Make sure you set limit based on the vGPU account to avoid tf-serving process occupy all the gpu memory
        - --per_process_gpu_memory_fraction=0.2
        env:
        - name: MODEL_NAME
          value: resnet
        ports:
        - containerPort: 8501
        resources:
          limits:
            eks.amazonaws.com/vgpu: 1
        volumeMounts:
          - name: nvidia-mps
            mountPath: /tmp/nvidia-mps
      volumes:
        - name: nvidia-mps
          hostPath:
            path: /tmp/nvidia-mps
```

> **WARNING:** *if you don't request GPUs when using the device plugin with NVIDIA images all
> the GPUs on the machine will be exposed inside your container.*

## Development

Please check [Development](./DEVELOPMENT.md) for more details.