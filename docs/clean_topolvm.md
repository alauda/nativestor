Clean Topolvm Cluster
=============

If you want to tear down the cluster and bring up a new one, be aware of the following resources that will need to be cleaned up.  


PersistenceVolumeClaim
----------

The PersistenceVolumeClaim that references Topolvm's StorageClass should be deleted firstly. If user delete pvc successfully, the topolvm will clean logical volume in the node.


TopolvmCluster
--------------

you should back up TopolvmCluster information before delete it because we will use it in the subsequent step.

```console
kubectl -n [namespace] get topolvmcluster [sample-name] -o yaml > tmp.yaml
```

After backing up topolvm cluster, then you can delete it.

```console
kubectl -n [namespace] delete topolvmcluster [sample-name]
```


VolumeGroup and Physical Volume
--------------

you can remove volume group and physical volume base on topolvm cluster info that you back up.

you should ssh the node and then exec the following cmd

```console
lvm vgremove [vg-name]
lvm pvremove [phycal-volume that belongs to the volume group]
```