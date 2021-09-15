package monitor

const PrometheusRule = `
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  annotations:
    alert.cpaas.io/notifications: '[]'
    alert.cpaas.io/rules.description: '{}'
    alert.cpaas.io/rules.disabled: '[]'
    alert.cpaas.io/rules.version: '6'
  labels:
    alert.cpaas.io/kind: TopolvmCluster
    alert.cpaas.io/name: topolvmcluster
    alert.cpaas.io/namespace: operators
    alert.cpaas.io/owner: System
    alert.cpaas.io/project: ''
    prometheus: kube-prometheus
  name: topolvm-alert
  namespace: operators
spec:
  groups:
    - name: general
      rules:
        - alert: volume.group.usage.near.full
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            display_name: Topolvm设备类容量已使用率
          expr: (topolvm_volumegroup_size_bytes - topolvm_volumegroup_available_bytes) / (topolvm_volumegroup_size_bytes )>0.75
          for: 5s
          labels:
            alert_indicator: custom
            alert_description: VolumeGroup {{ $labels.device_class }}  node {{ $labels.node }}  nodeutilization has crossed 75%. Free up some space or expand the VolumeGroup.
            alert_indicator_aggregate_range: '0'
            alert_indicator_blackbox_name: ''
            alert_indicator_comparison: '>'
            alert_indicator_query: ''
            alert_indicator_threshold: '0.75'
            alert_indicator_unit: ''
            alert_involved_topolvm_kind: Cluster
            alert_involved_object_kind: TopolvmCluster
            alert_involved_object_namespace: operators
            alert_involved_object_name: topolvm
            alert_name: volume.group.usage.near.full
            alert_project: ''
            alert_resource: topolvm-alert
            severity: Medium
        - alert: topolvm.cluster.unavailable
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            display_name: Topolvm集群状态不健康
          expr: topolvm_health_cluster_status!=0
          for: 60s
          labels:
            alert_indicator: custom
            alert_description: Topolvm cluster is unavailable
            alert_indicator_aggregate_range: '0'
            alert_indicator_blackbox_name: ''
            alert_indicator_comparison: '!='
            alert_indicator_query: ''
            alert_indicator_threshold: '0'
            alert_indicator_unit: ''
            alert_involved_topolvm_kind: Cluster
            alert_involved_object_kind: TopolvmCluster
            alert_involved_object_namespace: operators
            alert_involved_object_name: topolvm
            alert_name: topolvm.cluster.unavailable
            alert_project: ''
            alert_resource: topolvm-alert
            severity: Critical
        - alert:  volume.group.usage.critical
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            display_name: Topolvm设备类容量已使用率
          expr: (topolvm_volumegroup_size_bytes - topolvm_volumegroup_available_bytes) / (topolvm_volumegroup_size_bytes )>0.85
          for: 30s
          labels:
            alert_indicator: custom
            alert_description: VolumeGroup {{ $labels.device_class }}  node {{ $labels.node }} nodeutilization has crossed 85%. Free up some space or expand the VolumeGroup immediately.
            alert_indicator_aggregate_range: '0'
            alert_indicator_blackbox_name: ''
            alert_indicator_comparison: '>'
            alert_indicator_query: ''
            alert_indicator_threshold: '0.85'
            alert_indicator_unit: ''
            alert_involved_topolvm_kind: Cluster
            alert_involved_object_kind: TopolvmCluster
            alert_involved_object_namespace: operators
            alert_involved_object_name: topolvm
            alert_name:  volume.group.usage.critical
            alert_project: ''
            alert_resource: topolvm-alert
            severity: High
        - alert: topolvm.node.unavailable
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            display_name: Topolvm节点不健康
          expr: topolvm_health_node_status!=0
          for: 60s
          labels:
            alert_indicator: custom
            alert_description:  Topolvm node {{ $labels.node_name }} is unavailable
            alert_indicator_aggregate_range: '0'
            alert_indicator_blackbox_name: ''
            alert_indicator_comparison: '!='
            alert_indicator_query: ''
            alert_indicator_threshold: '0'
            alert_indicator_unit: ''
            alert_involved_topolvm_kind: Node
            alert_involved_object_kind: TopolvmCluster
            alert_involved_object_namespace: operators
            alert_involved_object_name: topolvm
            alert_name: topolvm.node.unavailable
            alert_project: ''
            alert_resource: topolvm-alert
            severity: Critical
`
