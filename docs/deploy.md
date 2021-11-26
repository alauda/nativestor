Deploying Topolvm-Operator
=============

Prepare Env
----------
How check and turn on `CSIStorageCapacity` please see [Storage Capacity](https://kubernetes.io/docs/concepts/storage/storage-capacity/)

Chart deploy
-----------
Chart package file see [deploy/chart](../deploy/chart). Follow the [documentation](https://helm.sh/docs/intro/using_helm/) to use helm deploy it into your Kubernetes cluster.


Yaml file deploy
------------
file exists in [deploy/example/](../deploy/example)
1. kubectl apply -f [operator.yaml](../deploy/example/operator.yaml)
2. kubectl apply -f [setting.yaml](../deploy/example/setting.yaml)

