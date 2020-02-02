### Forward Operator

[![pipeline status](https://gitlab.com/kainlite/forward/badges/master/pipeline.svg)](https://gitlab.com/kainlite/forward/commits/master)
[![coverage report](https://gitlab.com/kainlite/forward/badges/master/coverage.svg)](https://gitlab.com/kainlite/forward/commits/master)

This project aims to ease and do two things, connect to private resources safely and being simple enough like any other kubernetes resource, it relies in socat to do so (maybe at some point it will not), basically it will spin up a pod with socat and some given params to create the connection for you, at this time only the port-fordward method has been written for tcp and udp (udp needs more testing), tested and works. It uses the same port for the Pod that the remote connection uses.

There is a blog page describing how to get here, [check it out](https://techsquad.rocks/blog/cloud_native_applications_with_kubebuilder_and_kind_aka_kubernetes_operators/).

Also if you are interested how I got the idea to make this operator check this [github issue](https://github.com/kubernetes/kubernetes/issues/72597).

### Installation
To install this operator in your cluster you need to do the following:
```
make deploy IMG=kainlite/forward:0.0.1
```

### Why forward
I think this is probably the easiest way to adopt such a thing, and to put something like this into kubernetes itself sounds hard, and some people could resist, so I'm just trying to have an alternative but native to kubernetes, hence an operator.

### Security
Of course, this can make secure things insecure by exposing them, so use at your own risk and be aware of what you expose, how, and where...

### Use cases
Basically this should ease the life of a developer trying to reach a DB in a private subnet, or connect securely to a production endpoint to debug something, you name it, it only fills the gap that port-forward currently has.

## Option one:
Doing it manually without the controller, naked socat example:

```
kubectl run --restart=Never --image=alpine/socat TEMP_POD_NAME -- -d -d tcp-listen:PORT,fork,reuseaddr tcp-connect:HOSTNAME:PORT
kubectl port-forward pod/TEMP_POD_NAME LOCAL_PORT:PORT
```

Doing it with the operator, example resource:
```
apiVersion: forward.techsquad.rocks/v1beta1
kind: Forward
metadata:
  name: mapsample
  namespace: default
spec:
  host: 10.244.0.8
  port: 8000
  protocol: tcp
  liveness_probe: true
```

Then just do the port-forward:
```
kubectl port-forward pod/forward-privatedb-a LOCAL_PORT:PORT
```

It might be overkill to have a controller to wrap this, but it's the kubernetes way.

### How to get here
You don't need to do this:
```
# Create the project and also an API
kubebuilder init --domain techsquad.rocks
kubebuilder create api --group forward --version v1beta1 --kind Map
# Install the CRD and run the Controller to test
make install
make run
# Build the docker image, push it to the registry and deploy it
make docker-build docker-push IMG=kainlite/forward:0.0.1
make deploy IMG=kainlite/forward:0.0.1
# Uninstall the whole thing from the cluster
make uninstall
```

Manually testing, in one terminal, you will need to create a resource like the one from the example but with the alpine pod ip as host:
```
kubectl run -it --rm --restart=Never alpine --image=alpine sh
nc -l -p 8000
```

In another terminal:
```
kubectl port-forward forward-mapsample-pod 8000:8000
nc localhost 8000
```
Then type something and hit enter, it should show up in the first nc.

<a href="https://www.buymeacoffee.com/NDx5OFh" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/default-green.png" alt="Buy Me A Coffee" style="height: 51px !important;width: 217px !important;" ></a>
