User manual
==========

**Table of contents**

- [TopolvmCluster](#topolvmcluster)
- [StorageClass](#storageclass)
- [How to make pod be scheduler to specific node](#How-to-make-pod-be-scheduler-to-specific-node)
- [How to add device to volume group when space is not enough](#How-to-add-device-to-volume-group-when-space-is-not-enough)
- [How to use block pvc](#How-to-use-block-pvc)
- [How to use loop device](#How-to-use-loop-device)

TopolvmCluster
------------

A kubernetes cluster map to a `TopolvmCluster` instance. No support a kubernetes cluster has multi `TopolvmCluster` instances  

###Use all node and devices
An example of TopolvmCluster look like this:
```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: topolvm-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  storage:
    useAllNodes: true
    useAllDevices: true
    useLoop: false
    volumeGroupName: "hdd"
    className: "hdd"
```
`namespace` must be the same with the namespace of operator. one and only one class in a node must set `default` to true.   
a kubernetes cluster only existing a TopolvmCluster , not support multi TopolvmClusters.  
`topolvmVersion` topolvm image version, the image include csi sidecar.  
`useAllNodes` use all nodes of kubernetes cluster, default false.  
`useAllDevices` use all available devices of each node, default false.  
`useLoop` use loop devices present in nodes.  
`volumeGroupName` each node will create volume group.  
`className` used for classifying devices. such as hdd and ssd.  

###Use all node and specific device

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: topolvm-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  storage:
    useAllNodes: true
    # if you not want to use all devices of node, you should make it false, and define devices
    useAllDevices: false
    useLoop: false
    volumeGroupName: "hdd"
    className: "hdd"
    devices:
      - name: "/dev/sdb"
        type: "disk"
```
`devices` you can assign some devices to topolvm instead of using all available devices.  
note: if you want to use this case, you must set `useAllDevices` false

###Specific nodes and devices

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: topolvm-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  storage:
    useAllNodes: false
    useAllDevices: false
    useLoop: false
    deviceClasses:
      # kubernetes node name
      - nodeName: "192.168.16.98"
        # node classes
        classes:
          # node class name
          - className: "hdd"
            # user should specify volume group name , operator will create it
            volumeGroup: "hdd"
            # a node must a class should set default, when StorageClass not specific device class name , the default class will be used
            default: true
            # available devices used for creating volume group
            devices:
              - name: "/dev/sdb"
                type: "disk"
```
`deviceClasses` you can assign some nodes and devices to topolvm instead of using all nodes.
`classes` you can define multi classes up to your need, for example the node has ssd and hdd disk.

note: if you want to use this case, you must set `useAllNodes` false 

The class settings can be specified in the following fields:

| Name           | Type        | Default | Description                                                                        |
| -------------- | ------      | ------- | ---------------------------------------------------------------------------------- |
| `className`    | string      | -       | The name of a class.                                                               |
| `volumeGroup`  | string      | -       | The group where this class creates the logical volumes.                            | 
| `default`      | bool        | `false` | A flag to indicate that this device-class is used by default.                      |
| `devices`      | array/name  | -       | The available devices used for creating volume group                               |
| `devices.type` | string      | -       | the type of devices now can be support disk and loop                               |

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


How to add device to volume group when space is not enough
----------
if specific node available storage space is not enough, you can add device to volume group just edit `TopolvmCluster` instance.  

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  namespace: topolvm-system
spec:
  topolvmVersion: alaudapublic/topolvm:2.0.0
  storage:
    useAllNodes: false
    useAllDevices: false
    useLoop: false
    deviceClasses:
      - nodeName: "192.168.16.98"
        classes:
          - className: "hdd"
            volumeGroup: "hdd"
            default: true
            devices:
              - name: "/dev/sdb"
                type: "disk"
              #add new device
              - name: "/dev/sdc"
                type: "disk"
```

How to use block pvc
-----------
[PersistentVolumeClaim requesting a Raw Block Volume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistent-volume-claim-requesting-a-raw-block-volume)

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








