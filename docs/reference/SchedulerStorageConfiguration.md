# Scheduler Storage Configuration

Maestro supports two types of storage backends for schedulers:
- **PostgreSQL** (default) - Traditional relational database storage
- **Kubernetes CRDs** - Native Kubernetes Custom Resource Definitions

## Configuration via Environment Variables

The storage type and configuration can be set using environment variables. Maestro uses Viper, which automatically maps environment variables to configuration keys.

### Storage Type Selection

```bash
# Set the storage type (postgres or kubernetes)
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_TYPE="kubernetes"
```

### PostgreSQL Storage Configuration

When using PostgreSQL storage (default):

```bash
# PostgreSQL connection URL
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_POSTGRES_URL="postgres://username:password@host:5432/database?sslmode=disable"
```

### Kubernetes CRD Storage Configuration

When using Kubernetes CRD storage:

```bash
# Storage type
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_TYPE="kubernetes"

# Namespace where CRDs will be stored (default: "default")
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_KUBERNETES_NAMESPACE="maestro-schedulers"

# Use in-cluster configuration (default: false)
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_KUBERNETES_INCLUSTER="true"

# When not using in-cluster config, specify these:
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_KUBERNETES_MASTERURL="https://kubernetes.default.svc"
export MAESTRO_ADAPTERS_SCHEDULERSTORAGE_KUBERNETES_KUBECONFIG="/path/to/kubeconfig"
```

## Configuration via YAML

You can also configure the storage in the `config.yaml` file:

```yaml
adapters:
  schedulerStorage:
    # Storage type can be "postgres" or "kubernetes"
    type: "kubernetes"
    
    # PostgreSQL configuration
    postgres:
      url: "postgres://maestro:maestro@localhost:5432/maestro?sslmode=disable"
    
    # Kubernetes CRD configuration
    kubernetes:
      namespace: "maestro-schedulers"
      inCluster: true
      # Only used when inCluster is false
      masterUrl: "https://127.0.0.1:6443"
      kubeconfig: "./kubeconfig/kubeconfig.yaml"
```

## Kubernetes CRD Prerequisites

Before using Kubernetes CRD storage, you must apply the CRD definition:

```bash
kubectl apply -f internal/adapters/storage/kubernetes/crd/scheduler_crd.yaml
```

This creates the `roomschedulers.maestro.io` custom resource definition in your cluster.

## Viewing Schedulers with Kubernetes Storage

When using Kubernetes CRD storage, you can view schedulers directly using kubectl:

```bash
# List all schedulers
kubectl get roomschedulers -n maestro-schedulers

# Get detailed information about a scheduler
kubectl describe roomscheduler scheduler-name -n maestro-schedulers

# View scheduler as YAML
kubectl get roomscheduler scheduler-name -n maestro-schedulers -o yaml
```

## Version Management

The Kubernetes CRD storage implements an n-1 version strategy:
- The current version is stored in the main spec
- The previous version is stored in `spec.previousVersion`
- Only two versions are maintained (current and previous)
- Version switching updates the CRD by moving current to previous and setting the new version as current