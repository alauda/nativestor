E2E tests 
=====================================

This directory contains codes for end-to-end tests of TopoLVM-Operator.
Since the tests make use of [kind (Kubernetes IN Docker)][kind], this is called "e2e" test.

Setup environment
-----------------

1. Prepare Ubuntu machine.
2. [Install Docker CE](https://docs.docker.com/install/linux/docker-ce/ubuntu/#install-using-the-repository).
3. Add yourself to `docker` group.  e.g. `sudo adduser $USER docker`
4. Run `make setup`.


How to run tests
----------------

if you want to provide your own topolvm image, please edit the [cluster.yaml](./manifest/cluster.yaml), 
change the spec.topolvmVersion field

run as follows:

```console
make test
```

When tests fail, use `kubectl` to inspect the Kubernetes cluster.

Cleanup
-------

To stop Kubernetes, run `make shutdown-kind`.

[kind]: https://github.com/kubernetes-sigs/kind
