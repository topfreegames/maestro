# MIT License
#
# Copyright (c) 2021 TFG Co
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

#!/bin/sh

set -e

K3D_CLUSTER_NAME=${K3D_CLUSTER_NAME:-maestro-dev}
KUBECONFIG_OUT_DIR="/kubeconfig-out"
KUBECONFIG_FILE_PATH="$KUBECONFIG_OUT_DIR/.k3d-kubeconfig.yaml"
KUBECONFIG_TMP_FILE_PATH="$KUBECONFIG_FILE_PATH.tmp"

# Check if cluster exists
if ! k3d cluster get $K3D_CLUSTER_NAME > /dev/null 2>&1; then
  echo "INFO: k3d cluster '$K3D_CLUSTER_NAME' not found. Creating it now..."
  k3d cluster create $K3D_CLUSTER_NAME \
    --agents 1 \
    --port '38080:80@loadbalancer' \
    --api-port '127.0.0.1:6443' \
    --network 'k3d-maestro-dev' \
    --wait
  echo "INFO: k3d cluster '$K3D_CLUSTER_NAME' created successfully."
else
  # Check if running, start if not
  if ! k3d cluster get $K3D_CLUSTER_NAME --no-headers 2>/dev/null | grep -q 'running'; then
    echo "INFO: k3d cluster '$K3D_CLUSTER_NAME' exists but is not running. Starting it..."
    k3d cluster start $K3D_CLUSTER_NAME --wait
  else
    echo "INFO: k3d cluster '$K3D_CLUSTER_NAME' already exists and is running."
  fi
fi

echo "INFO: Exporting kubeconfig for cluster '$K3D_CLUSTER_NAME'..."
mkdir -p $KUBECONFIG_OUT_DIR

# Get the kubeconfig and create two versions:
# 1. Host-accessible version (for external tools)
# 2. Container-internal version (for kubectl inside the container)
k3d kubeconfig get $K3D_CLUSTER_NAME > $KUBECONFIG_TMP_FILE_PATH

# Create host-accessible version (external tools can use this)
sed 's/server: https:\/\/127.0.0.1:6443/server: https:\/\/k3d-maestro-dev-serverlb:6443/g' $KUBECONFIG_TMP_FILE_PATH > $KUBECONFIG_FILE_PATH

# Create container-internal version (for kubectl inside the container)
# This should point to the load balancer from within the k3d network
cp $KUBECONFIG_TMP_FILE_PATH /root/.kube/config
sed -i 's/server: https:\/\/127.0.0.1:6443/server: https:\/\/k3d-maestro-dev-serverlb:6443/g' /root/.kube/config

rm $KUBECONFIG_TMP_FILE_PATH

echo "INFO: Kubeconfig exported. k3d service ready and will keep running."
echo "INFO: Container-internal kubeconfig configured at /root/.kube/config for kubectl access"
echo " ---- KUBECONFIG START ---- "
cat $KUBECONFIG_FILE_PATH
echo " ---- KUBECONFIG END ---- "

# Keep container running
tail -f /dev/null
