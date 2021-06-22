# Brigade Prometheus: One-Step Monitoring for Brigade

Brigade Prometheus adds monitoring to a Brigade 2 installation.

Brigade 2 itself is currently in an _alpha_ state and remains under heavy
development, as such, the same is true for this add-on component.

## Introducing Brigade Prometheus

Brigade Prometheus uses a combination of exporting metrics from Brigade 2's SDK, creating a dimensional data model from these metrics through Prometheus, and displaying the metrics to the user through Grafana's dashboard UI. Brigade Prometheus handles the provisioning and setup of all three components for any Brigade operators who wish to expose metrics to their users with a simple helm installation.

## Getting Started

Comprehensive documentation will become available in conjunction with a beta
release. In the meantime, here is a little to get you started.

### Installing and Configuring Brigade 2 on a _Private_ Kubernetes Cluster

This project has a major dependency on Brigade 2, and requires a full installation of Brigade 2 to function. Install Brigade 2 with _default_ configuration:

```console
$ export HELM_EXPERIMENTAL_OCI=1
$ helm chart pull ghcr.io/brigadecore/brigade:v2.0.0-alpha.5
$ helm chart export ghcr.io/brigadecore/brigade:v2.0.0-alpha.5 -d ~/charts
$ kubectl create namespace brigade2
$ helm install brigade2 ~/charts/brigade --namespace brigade2
```

Expose the API server on your local cluster:

```console
$ kubectl --namespace brigade2 port-forward service/brigade2-apiserver 8443:443 &>/dev/null &
```

Log in as the "root" user, using the default root password `F00Bar!!!`. Be sure
to use the `-k` option to disregard issues with the self-signed certificate.

```console
$ brig login -k --server https://localhost:8443 --root
```

Once Brigade 2 is up and running on your cluster, create a service account, and give it READ permissions:

```console
$ brig sa create -i <id> -d <description>
$ brig role grant READER --service-account <id>
```

Save the service account token somewhere safe.

### Installing and Configuring Brigade Prometheus on a _Private_ Kubernetes Cluster

Since this add-on is still in heavy development, you will need to clone this repository to install Brigade Prometheus into your local Kubernetes cluster. Once the repository is cloned, open the `values.yaml` file, and paste the service account token into the `exporter.brigade.apiToken` field.

There are two methods of authentication you can choose from for logging into Grafana. 
    1. Option to use Grafana's built in user management system. The username and password for the admin account are specified in the `grafana.auth` fields, and the admin can handle user management using the Grafana UI.
    2. Option to use an nginx reverse proxy and a shared username/password to access Grafana in anonymous mode.

For option 1, set `grafana.auth.proxy` to false in `values.yaml`, and true for option 2.

Save the file, and run `make hack` from the project's root directory.

Once all three pods of the project are up and running, run the following command to expose the Grafana frontend:

```console
$ kubectl port-forward <brigade-prometheus-grafana pod name> 3000:<3000 for option 1, 80 for option 2> -n brigade-prometheus
```

Enter your supplied credentials. You can now access the Grafana dashboard!
