User manual
==========

**Table of contents**

- [TopolvmCluster](#topolvmcluster)
- [StorageClass](#storageclass)
- [How to make pod be scheduler to specific node](#How-to-make-pod-be-scheduler-to-specific-node)
- [How to add device to volume group when space is not enough](#How-to-add-device-to-volume-group-when-space-is-not-enough)
- [How to use block pvc](#How-to-use-block-pvc)
- [How to use loop device for developing](#How-to-use-loop-device-for-developing)
- [How to get available devices in your cluster](#How-to-get-available-devices-in-your-cluster)
- [How to create pvc snapshot](#How-to-create-pvc-snapshot)
- [How to enable and use raw device](#How-to-enable-and-use-raw-device)

TopolvmCluster
------------

A kubernetes cluster map to a `TopolvmCluster` instance. No support a kubernetes cluster has multi `TopolvmCluster` instances

### Use all node and devices
An example of TopolvmCluster look like this

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: nativestor-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  # certsSecret: mutatingwebhook
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
`certsSecret` optional attribute. Used to provide the name of a [TLS secret](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets) which will be used for secure topolvm-controller mutating webhook. If not provided a self-signed certificate will be generated automatically.
`useAllNodes` use all nodes of kubernetes cluster, default false.
`useAllDevices` use all available devices of each node, default false.
`useLoop` use loop devices present in nodes.
`volumeGroupName` each node will create volume group.
`className` used for classifying devices. such as hdd and ssd.

### Use all node and specific device

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: nativestor-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  # certsSecret: mutatingwebhook
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

### Specific nodes and devices

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: nativestor-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  # certsSecret: mutatingwebhook
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
kind: PersistentVolumeClaim
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
  namespace: nativestor-system
spec:
  topolvmVersion: alaudapublic/topolvm:2.0.0
  # certsSecret: mutatingwebhook
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


How to enable and use raw device
------------

### Enable raw device

update `topolvm-operator-setting` configmap.  set the field `RAW_DEVICE_ENABLE` be true.

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: topolvm-operator-setting
  namespace: nativestor-system
data:
#  KUBELET_ROOT_DIR: "/var/lib/kubelet"
#  OPERATOR_LOG_LEVEL: "INFO"
  RAW_DEVICE_ENABLE: "true"
  TOPOLVM_ENABLE: "true"
  RAW_DEVICE_IMAGE: "docker.io/alaudapublic/raw-device:v1.0.0"
#  TOPOLVM_IMAGE: "build-harbor.alauda.cn/acp/topolvm:v3.6.0"
#  # Set replicas for csi provisioner deployment.
#  CSI_PROVISIONER_REPLICAS: "2"
#  CSI_LOG_LEVEL: "DEBUG"
#  CSI_REGISTRAR_IMAGE: "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.3.0"
#  CSI_RESIZER_IMAGE: "k8s.gcr.io/sig-storage/csi-resizer:v1.3.0"
#  CSI_PROVISIONER_IMAGE: "k8s.gcr.io/sig-storage/csi-provisioner:v3.0.0"
#  CSI_SNAPSHOTTER_IMAGE: "k8s.gcr.io/sig-storage/csi-snapshotter:v4.2.0"
#  CSI_ATTACHER_IMAGE: "k8s.gcr.io/sig-storage/csi-attacher:v3.3.0"
#  CSI_LIVENESS_IMAGE: "k8s.gcr.io/sig-storage/livenessprobe:v2.4.0"
```

### raw device storageclass
crate raw device storageclass 

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: rawdevice-provisioner
provisioner: nativestor.alauda.io
volumeBindingMode: WaitForFirstConsumer
```
### create pvc 

`volumeMode` must be set to `Block`
```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-raw-device
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: rawdevice-provisioner
  volumeMode: Block
```
### create pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: raw-test
  labels:
    app.kubernetes.io/name: raw-test
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:20.04
      command: ["/usr/local/bin/pause"]
      volumeDevices:
        - devicePath: /dev/sdf
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: pvc-raw-device
```


How to use loop device for developing
----------

An example topolvmcluster instance like this

```yaml
apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  # namespace must be the same with topolvm-operator
  namespace: nativestor-system
spec:
  # Add fields here
  topolvmVersion: alaudapublic/topolvm:2.0.0
  # certsSecret: mutatingwebhook
  storage:
    useAllNodes: false
    useAllDevices: false
    useLoop: true
    deviceClasses:
      # kubernetes node name
      - nodeName: "192.168.16.98"
        # node classes
        classes:
          # node class name
          - className: "hdd"
            # user should specify volume group name , operator will create it
            volumeGroup: "test"
            # a node must a class should set default, when StorageClass not specific device class name , the default class will be used
            default: true
            # available devices used for creating volume group
            devices:
              - name: "myloop"
                type: "loop"
                # true means operator will create loop 
                auto: true
                #loop file director
                path: "/data"
                #unit is G
                size: 10
```
`provisioner` must be true because if this set true, operator will manage loop device preventing node restart.if node restart,operator will lostup loop again.  
`type` loop means that the device type is loop.  
`auto` users should set auto true if user want operator to create loop; if user provide loop device that created by user, user could ignore it.  
`path` if auto is true, user must provide the loop file that created from.  
`size` the size of loop, unit is G.  

How to get available devices in your cluster
--------------
After deploying topolvm-operator, user may don't know available devices in your kubernetes cluster. User could use auto discover devices to find available devices.
operator will create configmap that contains available devices info for every node. 
device info configmap loop like this:

```yaml

apiVersion: v1
data:
  devices: '[{"name":"dm-0","parent":"","hasChildren":false,"devLinks":"/dev/disk/by-id/dm-name-ceph--1d3409d7--1679--4cb3--951d--bd9a5c92d3fd-osd--data--2f495a7b--80b2--43d8--9385--19a61adf139a
    /dev/mapper/ceph--1d3409d7--1679--4cb3--951d--bd9a5c92d3fd-osd--data--2f495a7b--80b2--43d8--9385--19a61adf139a
    /dev/ceph-1d3409d7-1679-4cb3-951d-bd9a5c92d3fd/osd-data-2f495a7b-80b2-43d8-9385-19a61adf139a
    /dev/disk/by-id/dm-uuid-LVM-E6kgQ2jD0a2tc1uhf4viuTCE4eeDjNlWO1ocdR8BtWDQu0IuhKgIJmc8oBy6PCHk","size":107369988096,"uuid":"b8b6c02b-6420-4b20-b965-3a15a58731a4","serial":"","type":"lvm","rotational":true,"readOnly":false,"Partitions":null,"filesystem":"","vendor":"","model":"","wwn":"","wwnVendorExtension":"","empty":false,"real-path":"/dev/mapper/ceph--1d3409d7--1679--4cb3--951d--bd9a5c92d3fd-osd--data--2f495a7b--80b2--43d8--9385--19a61adf139a","kernel-name":"dm-0"}]'
kind: ConfigMap
metadata:
  creationTimestamp: "2021-10-19T08:55:13Z"
  labels:
    topolvm/lvmdconfig: lvmdconfig
    topolvm/node: 192.168.83.20
  name: lvmdconfig-192.168.83.20
  namespace: operators
  resourceVersion: "6009383"
  uid: ce602c94-cf1d-4dba-a7b6-01fd2df9ee32
```
`devices` is json string that contain available devices that look like this:
```json
[
  {
    "name": "loop0",
    "parent": "",
    "hasChildren": false,
    "devLinks": "",
    "size": 5368709120,
    "uuid": "e5e075eb-1495-448c-a826-17b413ea2d54",
    "serial": "",
    "type": "loop",
    "rotational": true,
    "readOnly": false,
    "Partitions": null,
    "filesystem": "",
    "vendor": "",
    "model": "",
    "wwn": "",
    "wwnVendorExtension": "",
    "empty": false,
    "real-path": "/dev/loop0",
    "kernel-name": "loop0"
  },
  {
    "name": "dm-0",
    "parent": "",
    "hasChildren": false,
    "devLinks": "/dev/disk/by-id/dm-name-ceph--1d3409d7--1679--4cb3--951d--bd9a5c92d3fd-osd--data--2f495a7b--80b2--43d8--9385--19a61adf139a /dev/mapper/ceph--1d3409d7--1679--4cb3--951d--bd9a5c92d3fd-osd--data--2f495a7b--80b2--43d8--9385--19a61adf139a /dev/disk/by-id/dm-uuid-LVM-E6kgQ2jD0a2tc1uhf4viuTCE4eeDjNlWO1ocdR8BtWDQu0IuhKgIJmc8oBy6PCHk /dev/ceph-1d3409d7-1679-4cb3-951d-bd9a5c92d3fd/osd-data-2f495a7b-80b2-43d8-9385-19a61adf139a",
    "size": 107369988096,
    "uuid": "9e37170e-dd32-4e5e-a1dd-de4c642a8ce5",
    "serial": "",
    "type": "lvm",
    "rotational": true,
    "readOnly": false,
    "Partitions": null,
    "filesystem": "",
    "vendor": "",
    "model": "",
    "wwn": "",
    "wwnVendorExtension": "",
    "empty": false,
    "real-path": "/dev/mapper/ceph--1d3409d7--1679--4cb3--951d--bd9a5c92d3fd-osd--data--2f495a7b--80b2--43d8--9385--19a61adf139a",
    "kernel-name": "dm-0"
  }
]
```

How to create pvc snapshot
----------
Firstly, user should deploy snapshot controller, follow the [article](https://kubernetes-csi.github.io/docs/snapshot-controller.html) to deploy. Then, follow the [Volume Snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) to know how to use snapshot.  