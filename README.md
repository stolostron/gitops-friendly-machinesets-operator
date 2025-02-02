# GitOps-Friendly MachineSets Operator

MachineSets created by the OpenShift installer include a random string of characters in their names. For example, after you deploy an OpenShift cluster on AWS, you may find three MachineSets named like this:

```
$ oc get machineset -n openshift-machine-api
NAME                                   DESIRED   CURRENT   READY   AVAILABLE   AGE
cluster-3af1-6cnhg-worker-us-east-2a   0         0                             2d5h
cluster-3af1-6cnhg-worker-us-east-2b   0         0                             2d5h
cluster-3af1-6cnhg-worker-us-east-2c   0         0                             2d5h
```

The three MachineSets start with the `cluster-3af1-6cnhg` prefix which is called an _infrastructure name_ and unfortunately is generated randomly by the OpenShift installer. These MachineSets are difficult to manage using GitOps as their names cannot be determined ahead of time. In addition to this, the same infrastructure name is also used in the definition of these MachineSets, for example:

<pre>
$ oc get machineset -n openshift-machine-api cluster-3af1-6cnhg-worker-us-east-2a -o yaml
apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  annotations:
    autoscaling.openshift.io/machineautoscaler: openshift-machine-api/<b>cluster-3af1-6cnhg</b>-worker-us-east-2a
  creationTimestamp: "2021-11-20T18:22:10Z"
  generation: 8
  labels:
    machine.openshift.io/cluster-api-cluster: <b>cluster-3af1-6cnhg</b>
  name: <b>cluster-3af1-6cnhg</b>-worker-us-east-2a
  namespace: openshift-machine-api
  
  ...
</pre>

## How Does the GitOps-Friendly MachineSets Operator Help?

The GitOps-Friendly MachineSets Operator is supposed to be installed right after the OpenShift cluster has been deployed (day 2). It helps in two steps:

1. The operator allows you to create MachineSets without the need to supply the cluster-specific infrastructure name. Instead, you insert a special token `INFRANAME` into your MachineSet definition. This special token will be replaced with the real infrastructure name right after you apply the manifest to the cluster.

2. As soon as the first node created by your MachineSet becomes available, the operator will scale the installer-provisioned MachineSets down to zero. These MachineSets cannot be managed by GitOps, so let's not use them at all.

![GitOps-Friendly MachineSets Operator](docs/images/gitops_friendly_machinesets_operator.png "GitOps-Friendly MachineSets Operator")

> :exclamation: The GitOps-Friendly MachineSets Operator is meant to be installed right after the OpenShift cluster deployment and before any critical workloads are running on the cluster. **The operator will scale the installer-provisioned MachineSets down to zero which will wipe out the respective worker Machines from the cluster.** This could disrupt the critical workloads running on the cluster. Future versions of the operator will allow disabling this behavior so that the operator can be safely deployed on existing OpenShift clusters.

The operator was tested on AWS and vSphere OpenShift clusters, however, it should work with any underlying infrastructure provider. The operator was tested on OpenShift 4.8.20. The operator is known to not work with OpenShift 4.7.x due to the operator-sdk used to built the operator likely not being compatible with OpenShift 4.7.x.

## Building Container Images (Optional)

This section provides instructions for building your custom operator images. If you'd like to deploy the operator using pre-built images, you can continue to the next section.

### Prerequisites

The operator is built using [operator-sdk](https://sdk.operatorframework.io/docs/). Make sure you have operator-sdk installed on your machine including the additional prerequisites as described in the [Installation Guide](https://sdk.operatorframework.io/docs/building-operators/golang/installation/).

### Building Images

Choose the operator version you want to build. See `git tag` for the list of available versions. Set the version variable:

```
$ VERSION=0.2.0
```

Check out the tag:

```
$ git checkout $VERSION
```

Set the custom image names. Replace the image names below with your own:

```
$ IMG=quay.io/noseka1/gitops-friendly-machinesets-operator:v$VERSION
$ IMAGE_TAG_BASE=quay.io/noseka1/gitops-friendly-machinesets-operator
```

Build operator image:

```
$ make docker-build IMG=$IMG
```

Push the finished operator image to the image registry:

```
$ podman push $IMG
```

Generate operator bundle artifacts:

```
$ make bundle IMG=$IMG CHANNELS=stable DEFAULT_CHANNEL=stable
```

Build bundle container image:

```
$ make bundle-build IMAGE_TAG_BASE=$IMAGE_TAG_BASE
```

Push bundle image to registry:

```
$ podman push $IMAGE_TAG_BASE-bundle:v$VERSION
```

Build catalog container image:

```
$ make catalog-build IMAGE_TAG_BASE=$IMAGE_TAG_BASE
```

Push catalog image to registry:

```
$ podman push $IMAGE_TAG_BASE-catalog:v$VERSION
```

## Deploying the Operator

If you'd like to deploy the operator using your custom built images, substitute the name of your catalog image in `deploy/gitops-friendly-machinesets-catsrc.yaml`:

```
$ sed -i "s#image:.*#image: $IMAGE_TAG_BASE-catalog:v$VERSION#" deploy/gitops-friendly-machinesets-catsrc.yaml
```

Alternatively, you can leverage the pre-built operator images to deploy the operator:

```
$ sed -i "s#image:.*#image: quay.io/noseka1/gitops-friendly-machinesets-operator-catalog:v0.2.0#" deploy/gitops-friendly-machinesets-catsrc.yaml
```

Deploy the operator:

```
$ oc apply -k deploy
```

## Creating MachineSets

Create a MachineSet specific to your underlying infrastructure provider. For example, a MachineSet for AWS and vSphere may look like the ones below. Note that all occurences of the infrastructure name are marked using the `INFRANAME` token. Operator will replace this `INFRANAME` token with the real infrastructure name after the MachineSet manifest is applied to the cluster.

Also note that you must add two _annotations_ that are required for the operator to take any action on the MachineSet and respective Machines:
1. Set `metadata.annotations.gitops-friendly-machinesets.redhat-cop.io/enabled: "true"`
2. Set `spec.template.metadata.annotations.gitops-friendly-machinesets.redhat-cop.io/enabled: "true"`

### Sample AWS MachineSet

<pre>
apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  <b>annotations:
    gitops-friendly-machinesets.redhat-cop.io/enabled: "true"</b>
  labels:
    machine.openshift.io/cluster-api-cluster: <b>INFRANAME</b>
  name: mymachineset
  namespace: openshift-machine-api
spec:
  replicas: 3
  selector:
    matchLabels:
      machine.openshift.io/cluster-api-cluster: <b>INFRANAME</b>
      machine.openshift.io/cluster-api-machineset: mymachineset
  template:
    metadata:
      <b>annotations:
        gitops-friendly-machinesets.redhat-cop.io/enabled: "true"</b>
      labels:
        machine.openshift.io/cluster-api-cluster: <b>INFRANAME</b>
        machine.openshift.io/cluster-api-machine-role: worker
        machine.openshift.io/cluster-api-machine-type: worker
        machine.openshift.io/cluster-api-machineset: mymachineset
    spec:
      metadata: {}
      providerSpec:
        value:
          ami:
            id: ami-03d9208319c96db0c
          apiVersion: awsproviderconfig.openshift.io/v1beta1
          blockDevices:
          - ebs:
              encrypted: true
              iops: 0
              kmsKey:
                arn: ""
              volumeSize: 120
              volumeType: gp2
          credentialsSecret:
            name: aws-cloud-credentials
          deviceIndex: 0
          iamInstanceProfile:
            id: <b>INFRANAME</b>-worker-profile
          instanceType: m5.xlarge
          kind: AWSMachineProviderConfig
          metadata:
            creationTimestamp: null
          placement:
            availabilityZone: us-east-2a
            region: us-east-2
          securityGroups:
          - filters:
            - name: tag:Name
              values:
              - <b>INFRANAME</b>-worker-sg
          subnet:
            filters:
            - name: tag:Name
              values:
              - <b>INFRANAME</b>-private-us-east-2a
          tags:
          - name: kubernetes.io/cluster/<b>INFRANAME</b>
            value: owned
          userDataSecret:
            name: worker-user-data
</pre>

### Sample vSphere MachineSet

<pre>
apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  <b>annotations:
    gitops-friendly-machinesets.redhat-cop.io/enabled: "true"</b>
  labels:
    machine.openshift.io/cluster-api-cluster: <b>INFRANAME</b>
  name: mymachineset
  namespace: openshift-machine-api
spec:
  replicas: 3
  selector:
    matchLabels:
      machine.openshift.io/cluster-api-cluster: <b>INFRANAME</b>
      machine.openshift.io/cluster-api-machineset: mymachineset
  template:
    metadata:
      <b>annotations:
        gitops-friendly-machinesets.redhat-cop.io/enabled: "true"</b>
      labels:
        machine.openshift.io/cluster-api-cluster: <b>INFRANAME</b>
        machine.openshift.io/cluster-api-machine-role: worker
        machine.openshift.io/cluster-api-machine-type: worker
        machine.openshift.io/cluster-api-machineset: mymachineset
    spec:
      metadata: {}
      providerSpec:
        value:
          apiVersion: vsphereprovider.openshift.io/v1beta1
          credentialsSecret:
            name: vsphere-cloud-credentials
          diskGiB: 120
          kind: VSphereMachineProviderSpec
          memoryMiB: 32768
          metadata:
            creationTimestamp: null
          network:
            devices:
            - networkName: OpenShift Network
          numCPUs: 4
          numCoresPerSocket: 2
          snapshot: ""
          template: <b>INFRANAME</b>-rhcos
          userDataSecret:
            name: worker-user-data
          workspace:
            datacenter: Datacenter
            datastore: datastore1
            folder: /Datacenter/vm/mycluster
            resourcePool: /Datacenter/host/Cluster/Resources
            server: photon-machine.lab.example.com
</pre>

## Managing MachineSets Using Argo CD

To allow Argo CD to sync the MachineSet manifests correctly, we need to instruct Argo CD to ignore the MachineSet modifications that were made by the GitOps-Friendly MachineSet Operator. We can use the `ignoreDifferences` configuration option as described in [Diffing Customization](https://argo-cd.readthedocs.io/en/stable/user-guide/diffing/). See the examples down below.

![Argo CD MachineSet](docs/images/argocd_machineset_synced.png "Argo CD MachineSet")

### Sample AWS Argo CD Application

<pre>
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: machineset-demo
  namespace: openshift-gitops
spec:
  destination:
    name: in-cluster
  project: default
  source:
    path: docs/samples/aws/manifests
    repoURL: https://github.com/noseka1/gitops-friendly-machinesets-operator
    targetRevision: master
  syncPolicy:
    automated:
      prune: false
      selfHeal: true
  <b>ignoreDifferences:
  - group: machine.openshift.io
    kind: MachineSet
    namespace: openshift-machine-api
    jsonPointers:
    - /metadata/labels/machine.openshift.io~1cluster-api-cluster
    - /spec/selector/matchLabels/machine.openshift.io~1cluster-api-cluster
    - /spec/template/metadata/labels/machine.openshift.io~1cluster-api-cluster
    - /spec/template/spec/providerSpec/value/iamInstanceProfile/id
    # The jqPathExpressions below don't seem to be supported by openshift-gitops 1.3.1 operator,
    # use these jsonPointers for the meantime:
    - /spec/template/spec/providerSpec/value/securityGroups/0/filters/0
    - /spec/template/spec/providerSpec/value/subnet/filters/0/values/0
    - /spec/template/spec/providerSpec/value/tags/0
    # These jqPathExpressions don't seem to work in openshift-gitops 1.3.1. They would be preferable
    # as they allow for more precise filtering.
    jqPathExpressions:
    - .spec.template.spec.providerSpec.value.securityGroups[].filters[] | select(.name == "tag:Name") | .values[0]
    - .spec.template.spec.providerSpec.value.subnet.filters[] | select(.name == "tag:Name") | .values[0]
    - .spec.template.spec.providerSpec.value.tags[0]</b>
</pre>

### Sample vSphere Argo CD Application

<pre>
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: machineset-demo
  namespace: openshift-gitops
spec:
  destination:
    name: in-cluster
  project: default
  source:
    path: docs/samples/vsphere/manifests
    repoURL: https://github.com/noseka1/gitops-friendly-machinesets-operator
    targetRevision: master
  syncPolicy:
    automated:
      prune: false
      selfHeal: true
  <b>ignoreDifferences:
  - group: machine.openshift.io
    kind: MachineSet
    namespace: openshift-machine-api
    jsonPointers:
    - /metadata/labels/machine.openshift.io~1cluster-api-cluster
    - /spec/selector/matchLabels/machine.openshift.io~1cluster-api-cluster
    - /spec/template/metadata/labels/machine.openshift.io~1cluster-api-cluster
    - /spec/template/spec/providerSpec/value/template</b>
</pre>

## Troubleshooting

Increase the operator log level to get more verbose logs. Get the installed csv:

```
$ oc get csv -n gitops-friendly-machinesets
NAME                                          DISPLAY                       VERSION   REPLACES   PHASE
gitops-friendly-machinesets-operator.v0.2.0   GitOps-Friendly MachineSets   0.2.0                Succeeded
```

Edit the csv:

```
$ oc edit csv -n gitops-friendly-machinesets gitops-friendly-machinesets-operator.v0.2.0
```

Find the command-line parameters passed to the operator and add `-zap-log-level=5` like this:

<pre>
             - args:
                - --health-probe-bind-address=:8081
                - --metrics-bind-address=127.0.0.1:8080
                - --leader-elect
                <b>- -zap-log-level=5</b>
                command:
                - /manager
</pre>

This change in csv will propagate to the `gitops-friendly-machinesets-controller-manager` deployment object and finally to the pod. After the operator pod restarts, you should see more verbose logs:

```
$ oc logs \
    -f \
    -n gitops-friendly-machinesets \
    -c manager \
    gitops-friendly-machinesets-controller-manager-f6b784bdb-8xts7
```

Remember to set the operator log level back after you are done troubleshooting.
