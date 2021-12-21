# Roadmap

- [Raw devices Implement](#Raw-devices-Implement)
- [Manage volume group that user created](#Manage-volume-group-that-user-created)
- [Data Migration and Backup](#Data-Migration-and-Backup)
- [Nodes and devices selector](#Node-and-devices-selector)
- [Enrich operator setting](#Enrich-operator-setting)
- [Operator SDK framework](#Operator-SDK-framework)
- [Metric optimization](#Metric-optimization)
- [TopolvmCluster status optimization](#TopolvmCluster-status-optimization)
- [PVC rollback](#PVC-rollback)

### Raw devices Implement
Not only lvm, operator could orchestrate csi driver that support raw devices. we should implement csi driver.

### Manage volume group that user created
volume group that created by user has available storage capacity, use could config CR `TopolvmCluster` and then operator could manage these volume groups.

### Data Migration and Backup
Make cronjob that do regular backup

### Node and devices selector
Enrich node and device selectors that makes deployment more flexible.

### Enrich operator setting
Now only support config kubelet root dir and enable discover devices, later we should add resource request and limit, log level etc.

### Operator SDK framework
Use operator sdk framework to refactor controller.

### Metric optimization
Collect status and capacity information about each component pouring into prometheus.

### TopolvmCluster status optimization
Add and optimize the progress and health status of the deployment process.

### PVC rollback
If node down, operator should make pvc rollback let pod rescheduled to another node.