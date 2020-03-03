## Resnet model inference on Virtual GPU

This is an example to deploy resnet model and invoke the client to get prediction result.

### Deploy the model

We already build resnet50 model in conatiner image `seedjeffwan/tensorflow-serving-gpu:resnet`. You can build your own image using [Dockefile](./Dockerfile).

```shell
$ kubectl apply -f resnet.yaml
```

### Prepare the client

Since we plan to use `ClusterIP` for the model service, we will create a client in the cluster to communication.

```shell
$ kubectl apply -f client.yaml
```

Enter python client pod we created.

```shell
$ kubectl exec -it python-client bash

$ apt update && apt install -y vim
```

Prepare model client, copy the scripts from [resnet_client.py](./resnet_client.py)

```shell
$ vim client.py
```

Invoke model prediction, the first call will take some time to warm up, reset of the calls will be stable.

```shell
$ python client.py
Prediction class: 286, avg latency: 26.615 ms
```