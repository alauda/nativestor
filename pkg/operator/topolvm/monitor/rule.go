package monitor

const PrometheusRule = `
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  labels:
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
            description: VolumeGroup {{ $labels.device_class }}  node {{ $labels.node }}  nodeutilization has crossed 75%. Free up some space or expand the VolumeGroup.
            summary: volume group usage near full
          expr: (topolvm_volumegroup_size_bytes - topolvm_volumegroup_available_bytes) / (topolvm_volumegroup_size_bytes )>0.75
          for: 5s
          labels:
            alert_description: VolumeGroup {{ $labels.device_class }}  node {{ $labels.node }}  nodeutilization has crossed 75%. Free up some space or expand the VolumeGroup.
            severity: Medium
        - alert: topolvm.cluster.unavailable
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            description: Topolvm cluster is unavailable.
            summary: cluster unavailable
          expr: topolvm_health_cluster_status!=0
          for: 60s
          labels:
            alert_description: Topolvm cluster is unavailable
            severity: Critical
        - alert:  volume.group.usage.critical
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            description: VolumeGroup {{ $labels.device_class }}  node {{ $labels.node }} nodeutilization has crossed 85%. Free up some space or expand the VolumeGroup immediately.
            summary: volume group usage critical
          expr: (topolvm_volumegroup_size_bytes - topolvm_volumegroup_available_bytes) / (topolvm_volumegroup_size_bytes )>0.85
          for: 30s
          labels:
            alert_description: VolumeGroup {{ $labels.device_class }}  node {{ $labels.node }} nodeutilization has crossed 85%. Free up some space or expand the VolumeGroup immediately.
            severity: High
        - alert: topolvm.node.unavailable
          annotations:
            alert_current_value: '{{ $value }}'
            alert_notifications: '[]'
            description: Topolvm node {{ $labels.node_name }} is unavailable
            summary: node unavailable
          expr: topolvm_health_node_status!=0
          for: 60s
          labels:
            alert_description:  Topolvm node {{ $labels.node_name }} is unavailable
            severity: Critical
`
