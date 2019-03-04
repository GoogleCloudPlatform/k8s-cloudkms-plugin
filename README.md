# Kubernetes KMS Plugin for Cloud KMS

This repo contains an implementation of a [Kubernetes KMS Plugin][k8s-kms-plugin] for [Cloud KMS][gcp-kms].

**If you are running on Kubernetes Engine (GKE), you do not need this plugin. You can enable [Application Layer encryption][gke-secrets-docs] in GKE (currently in beta) and GKE will manage the communication between GKE and KMS automatically.**


## Use with Compute Engine

If you are running Kubernetes on VMs, the configuration is logically divided into the following stages. These steps are specific to Compute Engine (GCE), but could be expanded to any VM-based machine.

1. Create Cloud KMS Keys
1. Grant service account IAM permissions on Cloud KMS keys
1. Deploy the KMS plugin binary to the Kubernetes master nodes
1. Create Kubernetes encryption configuration

### Assumptions

This guide makes a few assumptions:

* You have gcloud and the [Cloud SDK][cloud-sdk] installed locally. Alternatively you can run in [Cloud Shell][cloud-shell] where these tools are already installed.

* You have [billing enabled][gcp-billing] on your project. This project uses Cloud KMS, and you will not be able to use Cloud KMS without billing enabled on your project. Even if you are on a free trial, you will need to enable billing (your trial credits will still be used first).

* The Cloud KMS keys exist in the same project where the GCE VMs are running. This is not a hard requirement (in fact, you may want to separate them depending on your security posture and threat model adhering to the principle of separation of duties), but this guide assumes they are in the same project for simplicity.

* The VMs that run the Kubernetes masters run with [dedicated Service Account][dedicated-sa] to follow the principle of least privilege. The dedicated service account will be given permission to encrypt/decrypt data using a Cloud KMS key.

* The KMS plugin will share its security context with the underlying VM. Even though principle of least privilege advocates for a dedicated service account for the KMS plugin, doing so breaks the threat model since it forces you to store a service account key on disk. An attacker with access to an offline VM image would therefore decrypt the contents of etcd offline by leveraging the service account stored on the image. GCE VM service accounts are not stored on disk and therefore do not share this same threat vector.

### Set environment variables

Set the following environment variables to your values:

```sh
# The ID of the project. Please note that this is the ID, not the NAME of the
# project. In some cases they are the same, but they can be different. Run
# `gcloud projects list` and choose the value in "project id" column.
PROJECT_ID="<project id>"

# The FQDN email of the service account that will be attached to the VMs which
# run the Kubernetes master nodes.
SERVICE_ACCOUNT_EMAIL="<service-account email>"

# These values correspond to the KMS key. KMS crypto keys belong to a key ring
# which belong to a location. For a full list of locations, run
# `gcloud kms locations list`.
KMS_LOCATION="<location>"
KMS_KEY_RING="<key-ring name>"
KMS_CRYPTO_KEY="<crypto-key name>"
```

Here is an example (replace with your values):

```sh
PROJECT_ID="my-gce-project23"
SERVICE_ACCOUNT_EMAIL="kms-plugin@my-gce-project23@iam.gserviceaccount.com"
KMS_LOCATION="us-east4"
KMS_KEY_RING="my-keyring"
KMS_CRYPTO_KEY="my-key"
```

### Create Cloud KMS key

Create the key in Cloud KMS.

```sh
# Enable the Cloud KMS API (this only needs to be done once per project
# where KMS is being used).
$ gcloud services enable --project "${PROJECT_ID}" \
    cloudkms.googleapis.com

# Create the Cloud KMS key ring in the specified location.
$ gcloud kms keyrings create "${KMS_KEY_RING}" \
    --project "${PROJECT_ID}" \
    --location "${KMS_LOCATION}"

# Create the Cloud KMS crypto key inside the key ring.
$ gcloud kms keys create "${KMS_KEY_NAME}" \
    --project "${PROJECT_ID}" \
    --location "${KMS_LOCATION}" \
    --keyring "${KMS_KEY_RING}"
```

### Grant Service Account key permissions

Grant the dedicated service account permission to encrypt/decrypt data using the Cloud KMS crypto key we just created:

```sh
$ gcloud kms keys add-iam-policy-binding "${KMS_KEY_NAME}" \
    --project "${PROJECT_ID}" \
    --location "${KMS_LOCATION}" \
    --keyring "${KMS_KEY_RING}" \
    --member "serviceAccount:${SERVICE_ACCOUNT_EMAIL}" \
    --role "roles/cloudkms.cryptoKeyEncrypterDecrypter"
```

In addition to the IAM permissions, you also need to increase the oauth scopes on the VM. The following command assumes your VM master nodes require the `gke-default` scopes. If that is not the case, alter the command accordingly. Replace "my-master-instance" with the name of your VM:

```sh
$ gcloud compute instances set-service-account "my-master-instance" \
   --service-account "${SERVICE_ACCOUNT_EMAIL}" \
   --scopes "gke-default, https://www.googleapis.com/auth/cloudkms"
```

Restart any instances to pickup the scope changes:

```sh
$ gcloud compute instances stop "my-master-instance"
$ gcloud compute instances start "my-master-instance"
```


### Deploy the KMS plugin

There are a few options for deploying the KMS plugin into a Kubernetes cluster:

1. As a Docker image
1. As a Go binary
1. As a [static pod][k8s-static-pod]

For the sake of brevity, only the first scenario is covered. For **testing purposes only**, a public Docker image exists at `gcr.io/cloud-kms-lab/cloud-kms-plugin:dev`. **Do not use this image in a production environment.** Instead, build you own local copy and push the image to a repository of your choice:

1. Modify the `cmd/plugin/BUILD` file's `container_push` rule to point to your repository:

    ```sh
    container_push(
        name = "push",
        format = "Docker",
        image = ":k8s-cloud-kms-plugin-docker",
        registry = "gcr.io",
        repository = "my-company/cloud-kms-plugin",
        tag = "0.1",
    )
    ```

1. Build and push the image

    ```sh
    $ bazel run cmd/plugin:push
    ```

#### Pull image onto master VMs

**On the Kubernetes master VM**, run the following command to pull the Docker image. Replace the URL with your repository's URL:

```sh
# Replace this with your image
$ docker pull "gcr.io/cloud-kms-lab/cloud-kms-plugin:dev"
```

#### Test the interaction between KMS Plugin and CloudKMS

**On the Kubernetes master VM**, instruct the KMS plugin to perform a self-test. First, set some environment variables:

```sh
KMS_FULL_KEY="projects/${PROJECT_ID}/locations/${KMS_LOCATION}/keyRings/${KMS_KEY_RING}/cryptoKeys/${KMS_CRYPTO_KEY}"
SOCKET_DIR="/var/kms-plugin"
SOCKET_PATH="${SOCKET_DIR}/socket.sock"
PLUGIN_HEALTHZ_PORT="8081"
PLUGIN_IMAGE="gcr.io/cloud-kms-lab/cloud-kms-plugin:dev" # Replace with your value
```

Start the container:

```sh
$ docker run \
    --name="kms-plugin" \
    --network="host" \
    --detach \
    --rm \
    --volume "${SOCKET_DIR}:${SOCKET_DIR}:rw" \
    "${PLUGIN_IMAGE}" \
      /k8s-cloud-kms-plugin \
        --logtostderr \
        --integration-test="true" \
        --path-to-unix-socket="${SOCKET_PATH}" \
        --key-uri="${KMS_FULL_KEY}"
```

Probe the plugin on it's healthz port. You should expect the command to return "OK":

```sh
$ curl "http://localhost:${PLUGIN_HEALTHZ_PORT}/healthz?ping-kms=true"
```

Stop the container:

```sh
$ docker kill --signal="SIGHUP" kms-plugin
```

Finally, depending on your deployment strategy, configure the KMS plugin container to automatically boot at-startup. This can be done with systemd or an orchestration/configuration management tool.


### Configure kube-apiserver

Update the kube-apiserver's encryption configuration to point to the shared socket from the KMS plugin. If you changed the `SOCKET_DIR` or `SOCKET_PATH` variables above, update the `endpoint` in the configuration below accordingly:

```yaml
kind: EncryptionConfiguration
apiVersion: apiserver.config.k8s.io/v1
resources:
  - resources:
    - secrets
    providers:
    - kms:
        name: myKmsPlugin
        endpoint: unix:///var/kms-plugin/socket.sock
        cachesize: 100
   - identity: {}
```

More information about this file and configuration options can be found in the [Kubernetes KMS plugin documentation][k8s-kms-plugin].


## Learn more

* Read [Encrypting Kubernetes Secrets with Cloud KMS][blog-container-security]
* Read [GKE documentation for encrypting secrets][gke-secrets-docs]
* Watch [Turtles all the way down: Managing Kubernetes Secrets][video-turtles]



[gcp-kms]: https://cloud.google.com/kms
[k8s-kms-plugin]: https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/
[blog-container-security]: https://cloud.google.com/blog/products/containers-kubernetes/exploring-container-security-encrypting-kubernetes-secrets-with-cloud-kms
[gke-secrets-docs]: https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets
[video-turtles]: https://www.youtube.com/watch?v=rLHJZE2XKl8
[cloud-sdk]: https://cloud.google.com/sdk
[cloud-shell]: https://cloud.google.com/shell
[gcp-billing]: https://cloud.google.com/billing/docs/how-to/modify-project#enable_billing_for_a_new_project
[k8s-static-pod]: https://kubernetes.io/docs/tasks/administer-cluster/static-pod/
[dedicated-sa]: https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances
