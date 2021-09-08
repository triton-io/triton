# How to DEBUG

## DEBUG: start with process locally

**Setup host**

First, make sure `kube-apiserver` could connect to your local machine.
Second, make sure the `OpenKruise` has been installed in your cluster. If not, install it according to [this doc](https://openkruise.io/en-us/docs/installation.html).

Then, run triton locally
```bash
kubectl create ns triton-system
make install
make run
```


## DEBUG: start with Pod

The followings are the steps to debug triton controller manager locally using Pod. 

**Install docker**

Following the [official docker installation guide](https://docs.docker.com/get-docker/).

**Install minikube**

Follow the [official minikube installation guide](https://kubernetes.io/docs/tasks/tools/install-minikube/).

**Develop locally**

Make your own code changes and validate the build by running `make manager` and `make manifests` in triton directory.

**Deploy customized controller manager**

The new controller manager will be deployed via a `Deployment` to replace the default triton controller manager.
The deployment can be done by following steps assuming a fresh environment:

* Prerequisites: create new/use existing [dock hub](https://hub.docker.com/) account ($DOCKERID), and create a `triton` repository in it;
* step 1: `docker login` with the $DOCKERID account;
* step 2: `export IMG=<image_name>` to specify the target image name. e.g., `export IMG=$DOCKERID/triton:test`;
* step 3: `make docker-build` to build the image locally;
* step 4: `make docker-push` to push the image to dock hub under the `triton` repository;
* step 5: `export KUBECONFIG=<your_k8s_config>` to specify the k8s cluster config. e.g., `export KUBECONFIG=$~/.kube/config`;
* step 6: `make deploy IMG=${IMG}` to deploy your triton-controller-manager to the k8s cluster;

Tips:
* You can perform manual tests and use `kubectl logs <triton-pod-name> -n triton-system` to check controller logs for debugging, and you can see your `<triton-pod-name>` by applying `kubectl get pod -n triton-system`.