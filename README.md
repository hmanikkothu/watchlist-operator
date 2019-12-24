# watchlist-operator

```
$ go mod init my-watchlist.io
$ kubebuilder init --domain demo.my-watchlist.io
```


```
$ kubebuilder create api --group webapp --kind MyWatchlist --version v1
Create Resource [y/n]
y
Create Controller [y/n]
y
```


```
$ kubebuilder create api --group webapp --kind Redis --version v1
Create Resource [y/n]
y
Create Controller [y/n]
y
```

```
$ kubebuilder create api --group webapp --kind Frontend --version v1
Create Resource [y/n]
n
Create Controller [y/n]
n
```

```
$ make manifests
go: creating new go.mod: module tmp
go: finding sigs.k8s.io/controller-tools/cmd v0.2.4
go: finding sigs.k8s.io/controller-tools/cmd/controller-gen v0.2.4
go: finding sigs.k8s.io v0.2.4
.
.
.
```

```
$ kubectl create -f config/crd/bases/
$ kubectl create -f config/samples/webapp_v1_redis.yaml 
$ kubectl create -f config/samples/webapp_v1_mywatchlist.yaml 
```


```
$ make run
go: creating new go.mod: module tmp
go: finding sigs.k8s.io/controller-tools/cmd/controller-gen v0.2.4
go: finding sigs.k8s.io v0.2.4
go: finding sigs.k8s.io/controller-tools/cmd v0.2.4
/Users/xxx/bin/controller-gen object:headerFile=./hack/boilerplate.go.txt paths="./..."
go fmt ./...
go vet ./...
```


```
$ kubectl port-forward svc/mywatchlist-sample 7000:8080
```

#### get logs from side-car pod 
```
kubectl logs watchlist-operator-controller-manager-8549945dc7-f2qtb -n watchlist-operator-system manager

kubectl logs --follow pod/watchlist-operator-controller-manager-8549945dc7-zh67c -n watchlist-operator-system manager 

# note: manager is the container name of the sidecar, this is required in this case because there are two containers in the manager pod - manager and rbac-proxy
```

#### deploy from repo
```
make deploy IMG=<repo>/mywatchlist-operator:v1
```

#### clean-up
```
make undeploy IMG=<repo>/mywatchlist-operator:v1
```

#### get all
```
kubectl get all -n watchlist-operator-system
```