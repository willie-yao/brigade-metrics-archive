---
title: Ingress
description: How to configure gateways for Ingress and automatic TLS.
aliases:
  - /ingress.md
  - /topics/ingress.md
  - /intro/ingress.md
  - /intro/topics/ingress.md
---

# Using an ingress

So you've got yourself a Kubernetes cluster somewhere in the cloud and you want to expose all your Brigade gateways using a Kubernetes ingress, and you might also want to configure automatic TLS for each new ingress. This document will guide you through an opinionated way of achieving this. Note that you should be able to switch any component and replace it with something else.

## Requirements

To follow along, you will need:

- a Kubernetes cluster with a public endpoint - most cloud-provided Kubernetes services should be able to provide a public IP for your services.
- a domain name you own, with DNS records managed through a DNS provider (for example Cloudflare).
- [Helm](https://helm.sh/) correctly configured in your cluster.


## Deploying the Nginx Ingress Controller

First, we deploy an Nginx ingress controller - it will serve as the access point for all traffic intended for our cluster, allowing us to only expose a single service outside the cluster:

```
$ helm install stable/nginx-ingress --name nginx-ingress
```

Once deployed, this chart will create (among other things) a Kubernetes service of type `LoadBalancer` - which should be provided by your managed Kubernetes service:

```
$ kubectl get svc
NAME                            TYPE           CLUSTER-IP     EXTERNAL-IP    PORT(S)                      AGE
nginx-ingress-controller        LoadBalancer   10.0.27.38     13.69.59.166   80:31875/TCP,443:31071/TCP   3m54s
nginx-ingress-default-backend   ClusterIP      10.0.121.201   <none>         80/TCP                       3m54s
```

We will use the public IP to configure our DNS name. You can use any DNS provider - and in this example, we will use Cloudflare to add a new A Record. Note that you must use a domain name you have access to (in this case, we will use `*.kube.radu.sh` - that means this configuration can provide URLs such as `abc.kube.radu.sh`):

![Cloudflare DNS](https://docs.brigade.sh/img/cloudflare-dns.png)

At this point, you should be able to create Ingress objects that point to internal Kubernetes services, without exposing them. However, the ingress would not have a TLS certificate, and manually adding certificates for each one, while possible, is not ideal. This is where [`cert-manager`](https://github.com/jetstack/cert-manager) comes into play, which allows us to  _automatically provision and manage TLS certificates in Kubernetes_.

## Cert-Manager

Following the [official instructions on deploying `cert-manager` onto Kubernetes via Helm](https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html#installing-with-helm):

```
# Install the CustomResourceDefinition resources separately
kubectl apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.11/deploy/manifests/00-crds.yaml

# Create the namespace for cert-manager
kubectl create namespace cert-manager

# Add the Jetstack Helm repository
helm repo add jetstack https://charts.jetstack.io

# Update your local Helm chart repository cache
helm repo update

# Install the cert-manager Helm chart
helm install \
  --name cert-manager \
  --namespace cert-manager \
  --version v0.11.0 \
  jetstack/cert-manager
```

This created some Kubernetes Custom Resources Definitions (CRDs), and deployed the Helm chart. Among the CRDs created is one of type `ClusterIssuer` - when an ingress resource is annotated with a specific cluster issuer, this will request a TLS certificate from the [ACME server URL](https://letsencrypt.org/docs/client-options/) from the definition below, and before expiration, an email will be sent to the address defined below - so make sure to edit the file.

Let's Encrypt defines two tiers for emitting certificates - staging and production. Production certificates have rather strong [limit rates](https://letsencrypt.org/docs/rate-limits/), so in order to make sure your setup works, start with generating staging certificates, then move to production certificates.

Below you can find the definitions for each of the tiers:

```yaml
   apiVersion: cert-manager.io/v1alpha2
   kind: ClusterIssuer
   metadata:
    name: letsencrypt-staging
   spec:
     acme:
      # The ACME server URL
      server: https://acme-staging-v02.api.letsencrypt.org/directory
      # Email address used for ACME registration
      # Make sure to change this.
      email: user@example.com
      # Name of a secret used to store the ACME account private key
      privateKeySecretRef:
        name: letsencrypt-staging
      # Add a single challenge solver, HTTP01 using nginx
      solvers:
      # An empty 'selector' means that this solver matches all domains
      - selector: {}
        http01:
          ingress:
            class: nginx
```

This is the definition for the production issuer (also make sure to change the email used):

```yaml
   apiVersion: cert-manager.io/v1alpha2
   kind: ClusterIssuer
   metadata:
    name: letsencrypt-prod
   spec:
    acme:
      # The ACME server URL
      server: https://acme-v02.api.letsencrypt.org/directory
      # Email address used for ACME registration
      email: user@example.com
      # Name of a secret used to store the ACME account private key
      privateKeySecretRef:
        name: letsencrypt-prod
      # Add a single challenge solver, HTTP01 using nginx
      solvers:
      # An empty 'selector' means that this solver matches all domains
      - selector: {}
        http01:
          ingress:
            class: nginx
```

> Note: `ClusterIssuer` objects work across namespaces - if you want to limit generating certificates to a single namespace, use an `Issuer` object.

Now we need to create the cluster issuer objects defined above:

```
$ kubectl create -f staging-issuer.yaml
clusterissuer.certmanager.k8s.io/letsencrypt-staging created
$ kubectl create -f prod-issuer.yaml
clusterissuer.certmanager.k8s.io/letsencrypt-prod created
```

## Putting it all together

Continuing [the example from the `cert-manager` documentation](http://docs.cert-manager.io/en/latest/tutorials/acme/quick-start/index.html), let's check that everything works, and you can create an ingress with TLS - the following will create a deployment, service, and ingress using [the Kubernetes Up and Running Demo](https://github.com/kubernetes-up-and-running/kuard). Make sure to change the domain in the ingress object to the domain you own, and that you configured to use the IP of the Nginx ingress using your DNS provider (recall how for this we used `*.kube.radu.sh`):

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kuard
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: kuard
    spec:
      containers:
      - image: gcr.io/kuar-demo/kuard-amd64:1
        imagePullPolicy: Always
        name: kuard
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: kuard
spec:
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
  selector:
    app: kuard
---
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: kuar-demo
  annotations:
    kubernetes.io/ingress.class: "nginx"
    kubernetes.io/tls-acme: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    cert-manager.io/acme-challenge-type: http01
spec:
  tls:
  - hosts:
    - kuar-demo.kube.radu.sh
    secretName: kuar-demo-prod-tls
  rules:
  - host: kuar-demo.kube.radu.sh
    http:
      paths:
      - path: /
        backend:
          serviceName: kuard
          servicePort: 80
```

In theory, this will create a `Certificate` resource in your namespace, backed by Let's Encrypt:

```
$ kubectl get certificate
NAME                 READY   SECRET               AGE
kuar-demo-prod-tls   True    kuar-demo-prod-tls   9m
```

At this point, the host defined in your ingress configuration should be accessible, with a TLS certificate, and pointing to the demo:

![Kuard TLS](https://docs.brigade.sh/img/kuard-tls.png)

Make sure to change the email in the cluster issuer to your own, and configure the domain and host of the ingress to the ones you own.

> Make sure to delete the ingress after you are done with the demo, as it may expose sensitive information about your cluster.


## Brigade gateways and TLS ingress

Now that we have everything set up, it's time to deploy Brigade. Since we want to also expose at least one gateway publicly, we will use the generic gateway, which can be used to create Brigade jobs from arbitrary event sources, through webhooks.

> Note that you could deploy Brigade _before_ setting up `cert-manager` - however, if you need TLS certificates for a gateway, that gateway must be deployed after configuring `cert-manager`.

Here are the custom Helm values we are going to use:


```yaml
genericGateway:
  enabled: true
  registry: brigadecore
  name: brigade-generic-gateway
  service:
    name: brigade-generic-service
    type: ClusterIP
    externalPort: 8081
    internalPort: 8000
  serviceAccount:
    create: true
    name:
  ingress:
    enabled: true
    annotations:
      kubernetes.io/ingress.class: "nginx"
      kubernetes.io/tls-acme: "true"
      cert-manager.io/cluster-issuer: "letsencrypt-prod"
      cert-manager.io/acme-challenge-type: http01
    hosts:
      - brigade-events-test.kube.radu.sh
    paths:
    - /
    tls:
      - secretName: brigade-events
        hosts:
        - brigade-events-test.kube.radu.sh
```

Notice how the `annotations`, `hosts`, `paths` and `tls` sections are almost identical with the example we had before. If we install Brigade using these values, the same process that happened before for the demo application starts now - we will have an Ingress object, and `cert-manager` will make an `Order` and create a `Certificate` object using a certificate from LetsEncrypt. A couple of seconds later, we can verify the generic gateway is successfully deployed, and we have an HTTPS endpoint:

![Brigade Ingress](https://docs.brigade.sh/img/brigade-ingress.png)

Now we can follow the same process for any gateway we want to expose on the Internet.

## Setting up a Service Mesh to monitor Brigade using Linkerd

Brigade has a handful of services that we may want to monitor and capture stats from.   The two core services handling inbound requests are the API server
and the Kashti server.  Additionally, any number of optional gateways would be candidates for inclusion.

A service mesh is an infrastructure layer that can provide such analytics, oftentimes also bringing in load balancing, authentication, authorization and
circuit breaking functionality.  In this example, we'll be using [Linkerd](linkerd) as our service mesh and focus primarily on service observability and
traceability.

Here we show how the applicable Brigade components can be configured so that Linkerd can provide service data for us to use.

### Installing Linkerd

Follow Steps 0-4 in the [Getting Started Guide](https://linkerd.io/2/getting-started/) to install the Linkerd cli on to your machine and the Linkerd server
on to the Kubernetes cluster hosting Brigade.  Note that you may also opt to install/manage the server via the corresponding Helm chart.  Check out the
[Helm Install Instructions](https://linkerd.io/2/tasks/install-helm/) for further details.

### Integration with Brigade

Linkerd's term for integration with a given service is injection.  Note that the [inject CLI docs](https://linkerd.io/2/reference/cli/inject/) show how to inject
Linkerd globally on a namespace.  For Brigade, we don't wish to do so, as the namespace it is deployed in will surely consist of a large number of Brigade Job pods.
Since injection means a Linkerd sidecar container will be attached to all pods, and owing to how Kubernetes currently works, this would prevent said Jobs from ever terminating.

We only wish to inject Linkerd into Brigade's long-running components - and for this example we're most interested in the components that receive external requests: the
Brigade API server and Kashti (with gateways mentioned a little later).

Wiring up Linkerd is as simple as attaching an annotation (`linkerd.io/inject: enabled`) to each pod we're interested in.  Here, we add the annotations to
the Brigade API server and Kashti, via a Helm release upgrade (or install, if no release currently exists):

```
helm upgrade --install brigade-server brigade/brigade \
  --set api.podAnnotations."linkerd\.io/inject"=enabled \
  --set kashti.podAnnotations."linkerd\.io/inject"=enabled
```

Alternatively, these can be added to the chart values file and supplied instead.

In the `values.yaml` file:

```
...

kashti:
  enabled: true
  podAnnotations:
    linkerd.io/inject: enabled

...

api:
  enabled: true
  podAnnotations:
    linkerd.io/inject: enabled

...
```

Then we invoke the upgrade command like so:

```
helm upgrade --install brigade-server brigade/brigade -f values.yaml
```

### Monitoring Brigade

Now that we have Linkerd hooked up to the services we are interested in, we can check things out via Linkerd's dashboard.

```
linkerd dashboard &
```

This will launch both the Linkerd dashboard and default Grafana dashboard, opening up the former in your default browser.  We can now navigate to
the namespace we have Brigade installed into (here `default`) and confirm both of the services are healthy and running:

![Linkerd Dashboard Deployment Stats](https://docs.brigade.sh/img/linkerd-dashboard-1.png)

Be sure to explore all the dashboard has to offer, including the Grafana dashboard.  Check out the [Dashboard Doc](https://linkerd.io/2/features/dashboard/)
for further details.

Note that much of the same information displayed in the dashboard can be gleaned from the `linkerd` cli.  See the [CLI Reference](https://linkerd.io/2/reference/cli/)
for further info.

### Bonus round: Linkerd Ingress configuration for monitoring Gateways

The default configuration for Brigade gateways is to funnel traffic through an ingress controller, as seen above.  Therefore, in addition to adding
the pod annotation to each gateway we wish to monitor, we'll need to add a bit of additional configuration to each gateway's ingress resource.

Following the [Using Ingress](https://linkerd.io/2/tasks/using-ingress/#nginx) doc, we see what is needed for configuration when using the Nginx Ingress
Controller.  It boils down to adding an additional annotation to the ingress resource.

Taking the Brigade GitHub App Gateway as our example, we add the necessary config to the Helm chart values and then upgrade the release:

In the `values.yaml` file, we add the `linkerd.io/inject` entry under `podAnnotations` and the `nginx.ingress.kubernetes.io/configuration-snippet` entry under `ingress.annotations`:

```
...

podAnnotations:
  linkerd.io/inject: enabled

ingress:
  enabled: true
  annotations:
    kubernetes.io/ingress.class: "ngin"
    kubernetes.io/tls-acme: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    cert-manager.io/acme-challenge-type: http01
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_set_header l5d-dst-override $service_name.$namespace.svc.cluster.local:$service_port;
      grpc_set_header l5d-dst-override $service_name.$namespace.svc.cluster.local:$service_port;

...
```

Then we invoke the upgrade command like so:

```
helm upgrade --install brigade-gh-app brigade/brigade-github-app -f values.yaml
```

Navigating back to our dashboard, we then see the updated TCP stats for our gateway, perhaps after (re-)sending a few webhooks from our GitHub App page
to generate some inbound and outbound traffic:

![Linkerd Dashboard TCP Stats](https://docs.brigade.sh/img/linkerd-dashboard-2.png)

This just scratches the surface of what Linkerd can provide, but we already have a great way to monitor the health of our Brigade services
at a glance.

[linkerd]: https://linkerd.io/