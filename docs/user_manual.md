User manual
==========

**Table of contents**

- [TopolvmCluster](#topolvmcluster)
- [StorageClass](#storageclass)
- [Topology Scheduler](#How to make pod be scheduler to specific node)


TopolvmCluster
------------

An example of TopolvmCluster look like this:
```yaml
apiVersion: topolvm.cybozu.com/v1
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  namespace: topolvm-system
spec:
  # specific topolvm version
  topolvmVersion: harbor-b.alauda.cn/acp/topolvm:v3.3-4-g36b8bce
  deviceClasses:
    - nodeName: "192.168.16.98"
      classes:
        - className: "hdd"
          volumeGroup: "test"
          default: true
          devices:
            - name: "/dev/sdb"
            - name: "/dev/sdc"
        - className: "ssd"
          volumeGroup: "test1"
          default: false
          devices:
            - name: "/dev/sdd"
            - name: "/dev/sde"
    - nodeName: "192.168.16.99"
      classes:
        - className: "ssd"
          volumeGroup: "test"
          default: true
          devices:
            - name: "/dev/sdb"
```
`namespace` must be the same with the namespace of operator. one class of one node must set `default` to true.  
a kubernetes cluster only existing a TopolvmCluster , not support multi TopolvmClusters.
`topolvmVersion` topolvm image version, the image include csi sidecar.  
`nodeName` kubernetes cluster node name, the node has some available devices.  
`classes` you can define multi classes up to your need, for example the node has ssd and hdd disk

The class settings can be specified in the following fields:

| Name           | Type        | Default | Description                                                                        |
| -------------- | ------      | ------- | ---------------------------------------------------------------------------------- |
| `name`         | string      | -       | The name of a class.                                                               |
| `volumeGroup`  | string      | -       | The group where this class creates the logical volumes.                            |
| `spare-gb`     | uint64      | `1`    | Storage capacity in GiB to be spared.                                              |
| `default`      | bool        | `false` | A flag to indicate that this device-class is used by default.                      |
| `devices`      | array/name  | -       | The available devices used for creating volume group                               |
| `stripe`       | uint        | -       | The number of stripes in the logical volume.                                       |
| `stripe-size`  | string      | -       | The amount of data that is written to one device before moving to the next device. |



StorageClass
------------
An example StorageClass looks like this:


```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-ssd
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/device-class": "ssd"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

`provisioner` must be `topolvm.cybozu.com`.

`parameters` are optional.
To specify a filesystem type, give `csi.storage.k8s.io/fstype` parameter.
To specify a device-class name to be used, give `topolvm.cybozu.com/device-class` parameter.
If no `topolvm.cybozu.com/device-class` is specified, the default device-class is used.

Supported filesystems are: `ext4` and `xfs`.

`volumeBindingMode` can be either `WaitForFirstConsumer` or `Immediate`.
`WaitForFirstConsumer` is recommended because TopoLVM cannot schedule pods
wisely if `volumeBindingMode` is `Immediate`.

`allowVolumeExpansion` enables CSI drivers to expand volumes.


How to make pod be scheduler to specific node
--------------

if you want your deployment pod be scheduled to node "192.168.16.98", you can create a `StorageClass` that specific `"topolvm.cybozu.com/device-class": "hdd"`.  
example look like this:
```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-hdd
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/device-class": "hdd"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true

---
apiVersion: v1
metadata:
  name: hello
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner-hdd
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: topo-test
  labels:
    app: topo-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: topo-test
  template:
    metadata:
      labels:
        app: topo-test
    spec:
      containers:
        - name: topo-test
          image: ubuntu:18.04
          command: ["sleep"]
          args: ["3000000"]
          volumeMounts:
            - name: topo
              mountPath: /data/
      volumes:
        - name: topo
          persistentVolumeClaim:
            claimName: hello

```

How to use loop device 
----------

An example look like this:

```console
truncate --size=20G  tmp.img
losetup -f tmp.img
```

Suppose that you create loop device is /dev/loop1.  
if node reboot, the loop device will be lost, so you should losetup once again when node restart.
one of recommended solution is losetup loop device again before kubelet start.  
you can add one command to /etc/systemd/system/kubelet.service. see below:
```
ExecStartPre=/bin/bash -c 'losetup /dev/loop1 /your-path/tmp.img'
```








