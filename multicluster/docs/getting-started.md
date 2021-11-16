# Antrea Mulit-Cluster Controller Installation

## Prepare Antrea Multi-Cluster Image

For Antrea multi-cluster, there will be only one image `antrea/antrea-multicluster-controller:latest` 
for all controllers, you need to prepare a docker image before setup MCS component,
you can follow below steps to get the image ready on your local cluster.

1. Go to `antrea/multi-cluster` folder, run `make docker-build`, you will get a new image
  named `antrea/antrea-multicluster-controller:latest` locally.

### Load Antrea Multi-Cluster image to a K8 cluster
1. Run `docker save antrea/antrea-multicluster-controller:latest > antrea-mcs.tar` to save the image.
2. Copy the image file `antrea-mcs.tar` to the nodes of your local cluster
3. Run `docker load < antrea-mcs.tar` in each node of your local cluster.

### Load Antrea Multi-Cluster image to kind cluster
1. kind load docker-image antrea/antrea-multicluster-controller:latest --name=<kind-cluster-name>

## Install Mulit-Cluster Controller

### Installation in Leader Cluster

Run below command to apply global CRDs in leader cluster:

```
kubectl apply -f build/yamls/antrea-multicluster-leader-global.yml
```

Install MCS controller in leader cluster, since MCS controller is running as namespaced
deployment, you should create a namespace first, then apply the manifest with new namespace.
below are sample commands.

```
kubectl create ns antrea-mcs-ns
hack/generate-manifest.sh -l antrea-mcs-ns | kubectl apply -f -
```

### Installation in Member Cluster

You can simply run below command to install MCS controller to member cluster.
```
kubectl apply -f build/yamls/antrea-multicluster-member-only.yml
```

## ClusterSet

In an Antrea multi-cluster cluster set, there will be at least one leader cluster and two
member clusters. In the below examples we will use cluster set id `test-clusterset` which
has two member clusters with cluster id `test-cluster-east`, `test-cluster-west` and one
leader cluster with id `test-cluster-leader`.

### Setting up access to leader cluster
We first need to setup access from all member clusters into the leader cluster's API server.
We recommend creating one service account for each member for fine-grained access control.

For example:

1. Apply the following yaml in the leader cluster to setup access for
`test-cluster-east`.

```yml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: member-east-access-sa
  namespace: antrea-mcs-ns
---
apiVersion: v1
kind: Secret
metadata:
    name: member-east-access-token
    namespace: antrea-mcs-ns
    annotations:
      kubernetes.io/service-account.name: member-east-access-sa
type: kubernetes.io/service-account-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: member-east-access-rolebinding
  namespace: antrea-mcs-ns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: antrea-multicluster-member-cluster-role
subjects:
  - kind: ServiceAccount
    name: member-east-access-sa
    namespace: antrea-mcs-ns
```
2. Do the same for the other member `test-cluster-west`.
3. Now copy the access token into the respective member clusters. E.g.
```
# On leader cluster
kubectl get secret member-east-access-token -n antrea-mcs-ns -o yaml | grep -w -e '^apiVersion' -e '^data' -e '^metadata' -e '^ *name:' -e '^kind' -e crt -e 'token:' -e '^type' | sed -e 's/kubernetes.io\/service-account-token/mcs-custom/g' >  member-east-access-token.yml
# On test-cluster-east cluster
kubectl apply -f member-east-access-token.yml
```
4. Similarly copy the token for `test-cluster-west`.

### Setting up ClusterSet

All clusters in the cluster set need to use `ClusterClaim` to claim itself as a member
of a cluster set. A leader cluster will define `ClusterSet` which includes leader and
member clusters.

* Create below `ClusterClaim` and `ClusterSet` in the member cluster `test-cluster-east`.
  NOTE: Use the correct server address for the leader cluster below instead of `https://172.18.0.2:6443`

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: east-membercluster-id
  namespace: default
name: id.k8s.io
value: test-cluster-east
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: clusterset-id
  namespace: default
name: clusterSet.k8s.io
value: test-clusterset
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
    name: test-clusterset
    namespace: default
spec:
    leaders:
      - clusterID: test-cluster-leader
        secret: "member-east-access-token"
        server: "https://172.18.0.2:6443"
    members:
      - clusterID: test-cluster-east
    namespace: antrea-mcs-ns
```

* Create below `ClusterClaim` and `ClusterSet` in the member cluster `test-cluster-west`. 
  NOTE: Use the correct server address for the leader cluster below instead of `https://172.18.0.2:6443`

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: west-membercluster-id
  namespace: default
name: id.k8s.io
value: test-cluster-west
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: clusterset-id
  namespace: default
name: clusterSet.k8s.io
value: test-clusterset
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
    name: test-clusterset
    namespace: antrea-mcs-ns
spec:
    leaders:
      - clusterID: test-cluster-leader
        secret: "meber-west-access-token"
        server: "https://172.18.0.2:6443"
    members:
      - clusterID: test-cluster-west
    namespace: antrea-mcs-ns
```

* Create below `ClusterClaim` in the leader cluster `test-cluster-leader`.
 
```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: leadercluster-id
  namespace: antrea-mcs-ns
name: id.k8s.io
value: test-cluster-leader
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: clusterset-id
  namespace: antrea-mcs-ns
name: clusterSet.k8s.io
value: test-clusterset
```

* Create below `ClusterSet` in the leader cluster `test-cluster-leader`.

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
    name: test-clusterset
    namespace: antrea-mcs-ns
spec:
    leaders:
      - clusterID: test-cluster-leader
    members:
      - clusterID: test-cluster-east
        serviceAccount: "member-east-access-sa"
      - clusterID: test-cluster-west
        serviceAccount: "member-west-access-sa"
    namespace: antrea-mcs-ns
```

## Use MCS Custom Resource

After you set up a clusterset properly, you can simply add a `ServiceExport` resource
as below to export a `Service` from one member cluster to other members in the 
clusterset, you can update the name and namespace according to your local K8s Service.

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ServiceExport
metadata:
  name: nginx
  namespace: kube-system
```

For example, once you export the `kube-system/nginx` Service in member cluster `test-cluster-west`,
Antrea multi-cluster controller in member cluster will create two corresponding `ResourceExport` 
in the leader cluster, and the controller in leader cluster will do some computations and create
two `ResourceImport` contains all exported Service and Endpoints' information. you can check 
resources as below in leader cluster:

```sh
$kubectl get resourceexport
NAME                                        AGE
test-cluster-west-default-nginx-endpoints   7s
test-cluster-west-default-nginx-service     7s

$kubectl get resourceimport
NAME                      AGE
default-nginx-endpoints   99s
default-nginx-service     99s
```

then you can go to member cluster `test-cluster-east` to check new created 
`kube-system/nginx` Service and Endpoints by multi-cluster controller.

## Manual Test

You can also create some MCS resource manually if you need to do some testing
again `ResourceExport`, `ResourceImport` etc. below lists a few sample yamls 
for you to create some MCS custom resources.

* A `ServiceExport` example which will expose a Service named `nginx` in namespace
  `kube-system` in a member cluster, let's create it in both `test-cluster-west` 
  and `test-cluster-east` clusters.

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ServiceExport
metadata:
  name: nginx
  namespace: kube-system
```

* Create two `ResourceExports` examples which wrap a `ServiceExport` named `nginx`
  to Service type of `ResourceExport` and Endpoint type of `ResourceExport` to 
  represent the exposed `nginx` service for both `test-cluster-west` and `test-cluster-east`
  in the leader cluster `test-cluster-leader`.

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceExport
metadata:
  name: test-cluster-west-nginx-kube-system-service
  namespace: antrea-mcs-ns
spec:
  clusterID: test-cluster-west
  name: nginx
  namespace: kube-system
  kind: Service
  service:
    serviceSpec:
      ports: 
      - name: tcp80
        port: 80
        protocol: TCP
```

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceExport
metadata:
  name: test-cluster-west-nginx-kube-system-endpoints
  namespace: antrea-mcs-ns
spec:
  clusterID: test-cluster-west
  name: nginx
  namespace: kube-system
  kind: Endpoints
  endpoints:
    subsets:
    - addresses:
      - ip: 192.168.225.49
        nodeName: node-1
      - ip: 192.168.225.51
        nodeName: node-2
      ports:
      - name: tcp8080
        port: 8080
        protocol: TCP
```

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceExport
metadata:
  name: test-cluster-east-nginx-kube-system-service
  namespace: antrea-mcs-ns
spec:
  clusterID: test-cluster-east
  name: nginx
  namespace: kube-system
  kind: Service
  service:
    serviceSpec:
      ports: 
      - name: tcp80
        port: 80
        protocol: TCP
```

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceExport
metadata:
  name: test-cluster-east-nginx-kube-system-endpoints
  namespace: antrea-mcs-ns
spec:
  clusterID: test-cluster-east
  name: nginx
  namespace: kube-system
  kind: Endpoints
  endpoints:
    subsets:
    - addresses:
      - ip: 192.168.224.21
        nodeName: node-one
      - ip: 192.168.226.11
        nodeName: node-two
      ports:
      - name: tcp8080
        port: 8080
        protocol: TCP
```

* Create two `ResourceImport` examples which represent the `nginx` service in 
  namespace `kube-system` in the leader cluster `test-cluster-leader`.

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceImport
metadata:
  name: nginx-kube-system-service
  namespace: antrea-mcs-ns
spec:
  name: nginx
  namespace: kube-system
  kind: ServiceImport
  serviceImport:
    spec:
      ports: 
      - name: tcp80
        port: 80
        protocol: TCP
      type: ClusterSetIP
```

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceImport
metadata:
  name: nginx-kube-system-endpoints
  namespace: antrea-mcs-ns
spec:
  name: nginx
  namespace: kube-system
  kind: EndPoints
  endpoints:
    subsets:
    - addresses:
      - ip: 192.168.225.49
        nodeName: node-1
      - ip: 192.168.225.51
        nodeName: node-2
      ports:
      - name: tcp8080
        port: 8080
        protocol: TCP
    - addresses:
      - ip: 192.168.224.21
        nodeName: node-one
      - ip: 192.168.226.11
        nodeName: node-two
      ports:
      - name: tcp8080
        port: 8080
        protocol: TCP
```