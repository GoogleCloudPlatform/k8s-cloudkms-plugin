# Kubernetes KMS Plugin for Google CloudKMS

This repo contains an implementation of K8S KMS Plugin for Google CloudKMS, as described [here](https://docs.google.com/document/d/1S_Wgn-psI0Z7SYGvp-83ePte5oUNMr4244uanGLYUmw/edit?ts=59f965e1#heading=h.d26ktd3t9943).

If you are interested in an additional layer of encryption when running Kubernetes on Google Kubernetes Engine (GKE),
GKE provides this feature out-of-the-box (currently in beta). In other words, you don't need to build, deploy and manage this
plugin when running on GKE - GKE does all of this for you.  
* See this [blog](https://cloud.google.com/blog/products/containers-kubernetes/exploring-container-security-encrypting-kubernetes-secrets-with-cloud-kms) for more details  
* Here is a [link](https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets) to GKE's documentation for this feature
* We also gave a [talk](https://www.youtube.com/watch?v=rLHJZE2XKl8) about encryption of secrets in Kubernetes at Kubecon China 2017 

## Using KMS Plugin for CloudKMS when running Kubernetes on Google Compute Engine (GCE)
If you are running Kubernetes on GCE, instruction below should get you started.  
The configuration of the KMS Plugin for CloudKMS could be logically divided into the following stages:  
* Creating CloudKMS Keys
* Granting GCE's service account IAM permissions on CloudKMS keys
* Deploying KMS Plugin binary (or its docker image) to Kubernetes Masters
* Creating Kubernetes encryption configuration (this last step is common for all KMS Plugins, and is documented [here](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/#encrypting-your-data-with-the-kms-provider))

This guide makes several assumptions:
* The Virtual Machine (VM) on which Kubernetes Master is hosted runs in the security context of a dedicated Service Account (as opposed to the
[default GCE service account](https://cloud.google.com/compute/docs/access/service-accounts#compute_engine_default_service_account)).
For details on how to configure a GCE VM with a dedicated service account see this [link](https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances).
This is an important point since such accounts will be granted encrypt/decrypt permissions on CloudKMS keys and we would like to 
limit (as much as possible) the scope of these sensitive privileges.

* KMS Plugin will share the security context of the underlying GCE VM.  
At the time of this writing (Feb 2019), this is the best option for configuring CloudKMS Plugin on GCE (when compared with creating a dedicated 
Service Account for the plugin and making the exported service account key available to the plugin). Even though a dedicated service account 
would allow us to comply with the principle of least privilege (by delegating only CloudKMS permissions), doing so breaks the
threat model of KMS Plugin since this forces us to store the exported service account key on disk. Therefore, attackers 
in possession of an offline image of a K8S Master, could decrypt the content of etcd by leveraging the service account stored on the same image.  
How does GCE's service account avoid this issue? GCE's service accounts request their tokens at runtime from GCE's metadata. 
Therefore, tokens do need to be stored on disk.  
Note that GKE Security team is working on an approach where we will be able to create workload specific service account without 
the drawback of exporting key malarial to disk (stay tuned).

* CloudKMS Keys will be created in the same project where Kubernetes clusters run - this is just to keep things simple, but
there is no hard requirement for this. On the contrary, in production environments, we recommend to separate key management from
cluster management, thus conforming to the principle of separation of duties.

Before proceeding, I recommend setting the following environment variables (adjust them to your environment):
```bash
PROJECT='my-gcp-project'
# SA - Service Account
GCE__SA="cluster-1@m${PROJECT}.iam.gserviceaccount.com"
KEY_LOCATION='us-central1'
# KEY_RING is a like a folder for CloudKMS keys
KEY_RING='k8s'
KEY_NAME='cluster-1'
# Name of the GCE instance that hosts Kubernetes Master
MASTER_INSTANCE='cluster-1-master'
# GCE Zone
ZONE='us-central1-a'

```

### Creating CloudKMS Key

```bashif you could give this a quick read
gcloud config set project "${PROJECT}"
# Enable CloudKMS API
gcloud services enable cloudkms.googleapis.com

# Create CloudKMS Key Ring
gcloud kms keyrings create "${KEY_RING}" \
    --location ${KEY_LOCATION} \
    --project ${PROJECT}

# Create CloudKMS Key
gcloud kms keys create "${KEY_NAME}" \
    --location "${KEY_LOCATION}" \
    --keyring "${KEY_RING}" \
    --purpose encryption \
    --project "${PROJECT}"

```

### Granting GCE Service Account IAM permissions on the key
```bash
gcloud kms keys add-iam-policy-binding "${KEY_NAME}" \
  --location "${KEY_LOCATION}" \
  --keyring "${KEY_RING}" \
  --member serviceAccount:"${GCE_SA}" \
  --role roles/cloudkms.cryptoKeyEncrypterDecrypter \
  --project "${PROJECT}"

# In addition to IAM permissions we also need to grant cloudkms scope to the service account
# Note that the command below assumes that your master requires gke-default scopes. If this is not the case
# adjust --scopes argument accordingly.
# You can review the currently assigned scopes by running this command:
gcloud compute instances list --filter=name:"${MASTER_INSTANCE}" --flatten="serviceAccounts[].scopes[]" --format="csv(serviceAccounts.scopes.basename())"
# You will need to reboot the VM after applying this change.
gcloud compute instances set-service-account "${MASTER_INSTANCE}" \
   --service-account ${GCE_SA} \
   --scopes gke-default, https://www.googleapis.com/auth/cloudkms
```

### Deploying KMS Plugin onto your Kubernetes Master
There are three potential options on how you could deploy KMS Plugin onto your master:
1. As a go binary
2. As a docker image
3. As a [static pod](https://kubernetes.io/docs/tasks/administer-cluster/static-pod/) 
For the sake of brevity, only the second scenario will be described in this guide, but you should be able to adapt these 
instructions to your particular environment.

For testing purposes, we maintain the latest docker image of the plugin here: gcr.io/cloud-kms-lab/cloud-kms-plugin:dev.  
Do not use this image in production environment, rather build a local copy and push the result into the repository of 
your choice.
Here are the steps to accomplish this:
- Modify cmd/plugin/BUILD file (specifically the container_push rule) to point to your repository.
Example:
```build
container_push(
    name = "push",
    format = "Docker",
    image = ":k8s-cloud-kms-plugin-docker",
    registry = "gcr.io",
    repository = "my-company/cloud-kms-plugin",
    tag = "0.1",
)
```
- To push the image:
```bash
 bazel run cmd/plugin:push
```

#### Pull the image onto the Master
The remaining steps need to be performed on Kubernetes Master. Specifically, on a GCE VM that is running as 
a service account with the encrypt/decrypt privileges on our target CloudKMS key.
```bash
# Replace this with your image.
IMAGE='gcr.io/cloud-kms-lab/cloud-kms-plugin:dev'
docker pull "${IMAGE}"
```
#### Test the interaction between KMS Plugin and CloudKMS
We are at the point were we could instruct the KMS Plugin to perform a self-test.

```bash
FULL_KEY_NAME="projects/${PROJECT}/locations/${KEY_LOCATION}/keyRings/${KEY_RING}/cryptoKeys/${KEY_NAME}"
SOCKET_DIR="/var/kms-plugin"
SOCKET="${SOCKET_DIR}/socket.sock"
PLUGIN_HEALTHZ_PORT=8081
docker run -d --rm --name=kms-plugin \
    --network="host" \
    -v "${SOCKET_DIR}":"${SOCKET_DIR}":rw \
    "${IMAGE}" \
    /k8s-cloud-kms-plugin --logtostderr --integration-test=true --path-to-unix-socket="${SOCKET}" --key-uri="${KEY_NAME}"

# Plugin exposes healthz (by default) on port 8081, by sending the following curl command, the plugin will perform self-test  
# The self-test includes a "ping" to CloudKMS, where an encrypt/decrypt operation is performed on a test data. 
curl http://localhost:"${PLUGIN_HEALTHZ_PORT}"/healthz?ping-kms=true
# You should expect this command to return "OK".

# kill the container
docker kill --signal=SIGHUP kms-plugin

# Depending on your deployment strategy setup kms-plugin container to run at-startup.

```

### Configuring kube-apiserver for secrets encryption
At this stage in your deployment you should leverage K8S documentation on configuring KMS Plugins.
Concretely, follow steps on how to [create encryption configuration](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/#encrypting-your-data-with-the-kms-provider)
Make sure that endpoint property is what your configured for ${SOCKET} variable.
Example:
```yaml
kind: EncryptionConfiguration
apiVersion: apiserver.config.k8s.io/v1
resources:
  - resources:
    - secrets
    providers:
    - kms:
        name: myKmsPlugin
        endpoint: unix:///tmp/sock.sock
        cachesize: 100
   - identity: {}
```
