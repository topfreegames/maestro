maestro-helm
============

Maestro-helm creates [Maestro](https://github.com/topfreegames/maestro) stack on Kubernetes. 

### Installing maestro
You will need [helm](https://github.com/kubernetes/helm).
```
helm repo add tfgco http://helm.tfgco.com
helm repo update
helm install tfgco/maestro --namespace maestro --name maestro
```

### Installing maestro-cli
See instructions in last [release page](https://github.com/topfreegames/maestro-cli/releases).
