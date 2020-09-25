## Benchmark

If you would like to run the machine learning inference benchmark to evaluate the performance when running multiple pods on one GPU, you may following below steps to get the benchmark result:

1. Set environment variable with your AWS account ID and default region

  ```bash
  export ACCOUNT_ID=123456789012
  export AWS_DEFAULT_REGION=us-west-2
  ```

2. Build the tensorflow-benchmark image:

  ```bash
  $ cat <<EOF | docker build -t ${ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/tensorflow-benchmark:v1.15.2 -
  FROM alpine as intermediate
  LABEL stage=intermediate
  RUN apk update && \
      apk add --update git && \
      git clone https://github.com/tensorflow/benchmarks.git && \
      cd benchmarks && \
      git checkout cnn_tf_v1.15_compatible

  # Choose the base image for our final image
  FROM tensorflow/tensorflow:1.15.2-gpu
  RUN mkdir /opt/benchmarks
  COPY --from=intermediate /benchmarks /opt/benchmarks
  EOF
  ```

3. Create ECR repository, login to ECR and upload image to ECR

  ```bash
  aws ecr create-repository --repository-name tensorflow-benchmark
  $(aws ecr get-login --no-include-email)
  docker push ${ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/tensorflow-benchmark:v1.15.2
  ```

4. Run tensorflow benchmark jobs in parallel, you may change the model name to resnet101, inception3, vgg16, please refer to [benchmark](https://github.com/tensorflow/benchmarks) for more informaiton about the parameters.

  ```bash
  $ cat <<EOF | kubectl apply -f -
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: tf-benchmark
      spec:
        completions: 4
        parallelism: 4
        backoffLimit: 1
        template:
          spec:
            restartPolicy: Never
            hostIPC: true
            containers:
            - name: tf-benchmark
              image: ${ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/tensorflow-benchmark:v1.15.2
              args:
              - "python3"
              - "/opt/benchmarks/scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py"
              - "--data_name=imagenet"
              - "--model=resnet50"
              - "--num_batches=100"
              - "--batch_size=4"
              - "--num_gpus=1"
              - "--gpu_memory_frac_for_testing=0.2"
              resources:
                limits:
                  k8s.amazonaws.com/vgpu: 2
              volumeMounts:
              - name: nvidia-mps
                mountPath: /tmp/nvidia-mps
            volumes:
            - name: nvidia-mps
              hostPath:
                path: /tmp/nvidia-mps
      EOF
  ```

5. Wait for jobs to complete

  ```bash
  kubectl wait --for=condition=complete --timeout=30m job/tf-benchmark > /dev/null
  ```

6. Get result of each job

  ```bash
  $ for podName in $(kubectl get pods -l job-name=tf-benchmark --no-headers -o custom-columns=":metadata.name")
    do
        score=$(kubectl logs $podName | grep 'total images/sec: ' | sed -E 's/total\ images\/sec\:\ (.*)/\1/g')
        echo $score
    done
  ```

7. You may get benchmark result as following, it represent the detected images per second in each job.
  ```bash
  15.47
  15.46
  16.00
  15.58
  ```
