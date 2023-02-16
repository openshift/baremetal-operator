# Setup Development Environment With Already Configured OCP Hub Cluster

## Run BMO locally

- start with [dev-setup.md](./docs/dev-setup.md) for all the evn variables and anything else to run BMO locally.
- replace all `http` with `https` and include `IRONIC_INSECURE=true`. Final list of variables maybe look like below.
   ```shell
    export DEPLOY_KERNEL_URL=http://localhost:6181/images/ironic-python-agent.kernel;
    export DEPLOY_RAMDISK_URL=http://localhost:6180/images/ironic-python-agent.initramfs;
    export GO111MODULE=on;
    export GOFLAGS=;
    export IRONIC_ENDPOINT=https://localhost:6385/v1/;
    export IRONIC_INSECURE=true;
    export IRONIC_INSPECTOR_ENDPOINT=https://localhost:5050/v1/;
    export OPERATOR_NAME=baremetal-operator
   ```
- use kubernetes `port-forwarding` create a proper link with components BMO is dependent on. Below is how Ironic is linked (notice the port number that matches evn variable above). Repeat it for all the others endpoints and URLs you may need to connect to.
   ```shell
   kubectl port-forward metal3-xxx -n openshift-machine-api 6385:6385
   ```
    - tip: prepend `https_proxy=socks5://localhost:<PORT>` if the hub behind a jumphost.

## Build an image locally and push
```shell
docker buildx build -f Dockerfile.ocp  . -t <registry>/<user-name>/my-baremetal-operator:<version> --platform linux/amd64 --push
```
- for `registry` we generally use `quay.io`
- `buildx` is useful when dev env has a different architecture than where the cluster is running. E.g dev ARM64 and cluster on AMD64. Can drop this otherwise.

## Update config and deploy custom bmo image
1. scale down cvo, otherwise it revert any follow-up changes in configs.
    ```shell
    oc scale deployment cluster-version-operator -n openshift-cluster-version --replicas=0
    ```

2. Update configMap to point the custom build. As value of `baremetalOperator` as your image.
    ```yaml
    oc edit cm cluster-baremetal-operator-images

    apiVersion: v1
    data:
      images.json: |
        {
          ...
          "baremetalOperator": "<registry>/<user-name>/my-baremetal-operator:<version>",
          ...
        }
    ...
    ```
3. Delete cluster-baremetal-operator-xxx pod. This is auto update the metal3 pod once it restarts and reads the newly configured CM from step 2.
   ```shell
   # assuming current project set to openshift-machine-api
    oc delete pod cluster-baremetal-operator-xxx
   ```

## Good to know: 
- For increased permission modify `cluster-baremetal-operator`
    ```shell
    oc edit ClusterRole cluster-baremetal-operator
    ```