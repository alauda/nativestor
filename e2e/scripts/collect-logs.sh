#!/usr/bin/env bash

set -x

# User parameters
: "${CLUSTER_NAMESPACE:="topolvm-system"}"
: "${LOG_DIR:="test"}"

LOG_DIR="${LOG_DIR%/}" # remove trailing slash if necessary
mkdir -p "${LOG_DIR}"

for pod in $(kubectl -n "${CLUSTER_NAMESPACE}" get pod -o jsonpath='{.items[*].metadata.name}'); do
  kubectl -n "${CLUSTER_NAMESPACE}" describe pod "$pod" > "${LOG_DIR}"/pod-describe-"$pod".txt
done
for dep in $(kubectl -n "${CLUSTER_NAMESPACE}" get deploy -o jsonpath='{.items[*].metadata.name}'); do
  kubectl -n "${CLUSTER_NAMESPACE}" describe deploy "$dep" > "${LOG_DIR}"/deploy-describe-"$dep".txt
  kubectl -n "${CLUSTER_NAMESPACE}" logs deploy/"$dep" --all-containers > "${LOG_DIR}"/deploy-describe-"$dep"-log.txt
done
for ds in $(kubectl -n "${CLUSTER_NAMESPACE}" get daemonset -o jsonpath='{.items[*].metadata.name}'); do
  kubectl -n "${CLUSTER_NAMESPACE}" describe daemonset "$ds" > "${LOG_DIR}"/daemonset-describe-"$ds".txt
  kubectl -n "${CLUSTER_NAMESPACE}" logs daemonset/"$ds" --all-containers > "${LOG_DIR}"/daemonset-describe-"$ds"-log.txt
done

for job in $(kubectl -n "${CLUSTER_NAMESPACE}" get job -o jsonpath='{.items[*].metadata.name}'); do
  kubectl -n "${CLUSTER_NAMESPACE}" describe job "$job" > "${LOG_DIR}"/job-describe-"$job".txt
  kubectl -n "${CLUSTER_NAMESPACE}" logs job/"$job" --all-containers > "${LOG_DIR}"/job-describe-"$job"-log.txt
done

kubectl -n "${CLUSTER_NAMESPACE}" get pods -o wide > "${LOG_DIR}"/cluster-pods-list.txt
kubectl get all -n "${CLUSTER_NAMESPACE}" -o wide > "${LOG_DIR}"/cluster-wide.txt
kubectl get all -n "${CLUSTER_NAMESPACE}" -o yaml > "${LOG_DIR}"/cluster-yaml.txt
kubectl -n "${CLUSTER_NAMESPACE}" get topolvmcluster -o yaml > "${LOG_DIR}"/topolvmcluster.txt
kubectl -n "${CLUSTER_NAMESPACE}" get cm -o yaml > "${LOG_DIR}"/configmap.txt
sudo lsblk | sudo tee -a "${LOG_DIR}"/lsblk.txt
journalctl -o short-precise --dmesg > "${LOG_DIR}"/dmesg.txt
journalctl > "${LOG_DIR}"/journalctl.txt