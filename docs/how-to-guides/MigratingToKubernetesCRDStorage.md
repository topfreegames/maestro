# Migrating from PostgreSQL to Kubernetes CRD Storage

This guide walks you through migrating your Maestro scheduler storage from PostgreSQL to Kubernetes CRDs.

## Prerequisites

- Maestro instance running with PostgreSQL storage
- Access to Kubernetes cluster where Maestro is deployed
- kubectl configured to access the cluster
- Maestro Management API accessible

## Migration Overview

The migration process involves:
1. Preparing the Kubernetes environment
2. Exporting schedulers from PostgreSQL
3. Deploying a temporary Maestro instance with Kubernetes storage
4. Importing schedulers to Kubernetes CRDs
5. Switching production to Kubernetes storage

## Step-by-Step Migration

### Step 1: Apply the CRD Definition

First, apply the RoomScheduler CRD to your Kubernetes cluster:

```bash
kubectl apply -f internal/adapters/storage/kubernetes/crd/scheduler_crd.yaml
```

Verify the CRD is created:

```bash
kubectl get crd roomschedulers.maestro.io
```

### Step 2: Create Migration Script

Create a migration script `migrate-schedulers.sh`:

```bash
#!/bin/bash

# Configuration
MAESTRO_API_URL="http://localhost:8080"  # Current Maestro API with PostgreSQL
MAESTRO_K8S_API_URL="http://localhost:8081"  # Temporary Maestro API with K8s storage
OUTPUT_DIR="./scheduler-backup"

# Create backup directory
mkdir -p $OUTPUT_DIR

echo "Step 1: Fetching all schedulers from PostgreSQL storage..."

# Get list of all schedulers
SCHEDULERS=$(curl -s "$MAESTRO_API_URL/api/v1/schedulers" | jq -r '.schedulers[].name')

if [ -z "$SCHEDULERS" ]; then
    echo "No schedulers found or error connecting to API"
    exit 1
fi

echo "Found schedulers: $SCHEDULERS"

# Export each scheduler
for SCHEDULER in $SCHEDULERS; do
    echo "Exporting scheduler: $SCHEDULER"
    
    # Get scheduler details
    curl -s "$MAESTRO_API_URL/api/v1/schedulers/$SCHEDULER" > "$OUTPUT_DIR/$SCHEDULER.json"
    
    # Get all versions
    VERSIONS=$(curl -s "$MAESTRO_API_URL/api/v1/schedulers/$SCHEDULER/versions" | jq -r '.versions[].version')
    
    # Save versions info
    echo "$VERSIONS" > "$OUTPUT_DIR/$SCHEDULER.versions"
done

echo "Step 2: Waiting for Kubernetes storage Maestro to be ready..."
echo "Please start the temporary Maestro instance with Kubernetes storage and press Enter"
read -p "Press Enter to continue..."

echo "Step 3: Importing schedulers to Kubernetes storage..."

# Import each scheduler
for SCHEDULER in $SCHEDULERS; do
    echo "Importing scheduler: $SCHEDULER"
    
    # Read the scheduler JSON
    SCHEDULER_JSON=$(cat "$OUTPUT_DIR/$SCHEDULER.json")
    
    # Create scheduler in Kubernetes storage
    curl -X POST \
        -H "Content-Type: application/json" \
        -d "$SCHEDULER_JSON" \
        "$MAESTRO_K8S_API_URL/api/v1/schedulers"
    
    echo "Scheduler $SCHEDULER imported"
done

echo "Migration completed!"
echo "Verify the schedulers in Kubernetes:"
echo "kubectl get roomschedulers -n <namespace>"
```

Make the script executable:

```bash
chmod +x migrate-schedulers.sh
```

### Step 3: Deploy Temporary Maestro with Kubernetes Storage

Create a temporary deployment configuration `maestro-k8s-temp.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: maestro-k8s-config
data:
  config.yaml: |
    api:
      port: 8081  # Different port to avoid conflict
    adapters:
      schedulerStorage:
        type: "kubernetes"
        kubernetes:
          namespace: "default"
          inCluster: true
      # Copy other adapter configurations from your current setup
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: maestro-k8s-temp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: maestro-k8s-temp
  template:
    metadata:
      labels:
        app: maestro-k8s-temp
    spec:
      containers:
      - name: maestro
        image: your-maestro-image:latest
        command: ["maestro", "start", "-c", "/config/config.yaml"]
        ports:
        - containerPort: 8081
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: maestro-k8s-config
```

Deploy the temporary instance:

```bash
kubectl apply -f maestro-k8s-temp.yaml
```

### Step 4: Run the Migration

Execute the migration script:

```bash
./migrate-schedulers.sh
```

### Step 5: Verify Migration

Check that all schedulers are migrated:

```bash
# List all schedulers in Kubernetes
kubectl get roomschedulers

# Compare with PostgreSQL count
psql -U maestro -d maestro -c "SELECT COUNT(*) FROM schedulers;"
```

Verify a specific scheduler:

```bash
# Get scheduler details
kubectl get roomscheduler <scheduler-name> -o yaml

# Compare with API response
curl http://localhost:8081/api/v1/schedulers/<scheduler-name>
```

### Step 6: Update Production Configuration

Update your production Maestro configuration to use Kubernetes storage:

```yaml
adapters:
  schedulerStorage:
    type: "kubernetes"
    kubernetes:
      namespace: "default"  # Or your preferred namespace
      inCluster: true
```

### Step 7: Deploy Updated Configuration

1. Update your Maestro deployment with the new configuration
2. Perform a rolling update to switch to Kubernetes storage
3. Monitor logs for any issues

```bash
kubectl rollout restart deployment/maestro
kubectl rollout status deployment/maestro
```

### Step 8: Clean Up

After verifying everything works correctly:

1. Remove the temporary Maestro deployment:
   ```bash
   kubectl delete -f maestro-k8s-temp.yaml
   ```

2. Archive the PostgreSQL database (keep as backup)

3. Remove scheduler backup files:
   ```bash
   rm -rf ./scheduler-backup
   ```

## Rollback Plan

If issues occur during migration:

1. Stop the migration process
2. Revert Maestro configuration to use PostgreSQL
3. Redeploy Maestro with PostgreSQL configuration
4. Investigate and fix issues before retry

## Post-Migration Considerations

### Monitoring

- Monitor Maestro logs for any storage-related errors
- Check Kubernetes events for CRD-related issues
- Verify scheduler operations (create, update, delete) work correctly

### Backup Strategy

With Kubernetes CRD storage, schedulers are backed up as part of your Kubernetes cluster backup. Ensure your backup solution includes CRDs.

### Performance

Kubernetes CRD storage may have different performance characteristics:
- List operations might be slower for large numbers of schedulers
- Updates are subject to Kubernetes API rate limits
- No need for separate database maintenance

## Troubleshooting

### Common Issues

1. **Permission Errors**
   - Ensure Maestro has RBAC permissions to manage CRDs
   - Check ServiceAccount permissions

2. **CRD Not Found**
   - Verify CRD is applied: `kubectl get crd roomschedulers.maestro.io`
   - Check namespace permissions

3. **Version Mismatch**
   - Kubernetes storage only maintains n-1 versions
   - Older versions won't be migrated

### RBAC Configuration

Ensure Maestro has proper permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: maestro-scheduler-manager
rules:
- apiGroups: ["maestro.io"]
  resources: ["roomschedulers"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: maestro-scheduler-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: maestro-scheduler-manager
subjects:
- kind: ServiceAccount
  name: maestro
  namespace: default
```