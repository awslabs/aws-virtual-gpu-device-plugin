FROM tensorflow/serving:1.15.0-gpu

RUN apt update && apt install curl

RUN mkdir -p /models/resnet

RUN curl -s https://storage.googleapis.com/download.tensorflow.org/models/official/20181001_resnet/savedmodels/resnet_v2_fp32_savedmodel_NHWC_jpg.tar.gz | tar --strip-components=2 -C /models/resnet -xvz