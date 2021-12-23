Deploying NativeStor
=============

Prepare Env
----------
make sure k8s version >=1.21

Deploy
------------
file exists in [deploy/example/](../deploy/example)
1. kubectl apply -f [operator.yaml](../deploy/example/operator.yaml)
2. kubectl apply -f [setting.yaml](../deploy/example/setting.yaml)



Operator setting
-------------

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: topolvm-operator-setting
  namespace: nativestor-system
data:
  KUBELET_ROOT_DIR: "/var/lib/kubelet"
  OPERATOR_LOG_LEVEL: "INFO"
  RAW_DEVICE_ENABLE: "true"
  TOPOLVM_ENABLE: "true"
  RAW_DEVICE_IMAGE: "docker.io/alaudapublic/raw-device:v1.0.0"
  TOPOLVM_IMAGE: "build-harbor.alauda.cn/acp/topolvm:v3.6.0"
  # Set replicas for csi provisioner deployment.
  CSI_PROVISIONER_REPLICAS: "2"
  CSI_LOG_LEVEL: "3"
  CSI_REGISTRAR_IMAGE: "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.3.0"
  CSI_RESIZER_IMAGE: "k8s.gcr.io/sig-storage/csi-resizer:v1.3.0"
  CSI_PROVISIONER_IMAGE: "k8s.gcr.io/sig-storage/csi-provisioner:v3.0.0"
  CSI_SNAPSHOTTER_IMAGE: "k8s.gcr.io/sig-storage/csi-snapshotter:v4.2.0"
  CSI_ATTACHER_IMAGE: "k8s.gcr.io/sig-storage/csi-attacher:v3.3.0"
  CSI_LIVENESS_IMAGE: "k8s.gcr.io/sig-storage/livenessprobe:v2.4.0"
```
If you want to use topolvm you should set `TOPOLVM_ENABLE` to be `true`  
if you want to use rawdevice you should set `RAW_DEVICE_ENABLE` to be `true`.  
The configmap namespace should be the same with operator deployment.  

User manual
---------------
please see [user mannual](./user_manual.md)