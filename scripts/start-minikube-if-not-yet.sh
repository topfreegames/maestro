#!/bin/bash

if [ ! $(which kubectl) ]; then
  if [ -z "$MAESTRO_TEST_CI" ]; then
    read -p "Kubectl not installed. Wish to install and proceed with integration tests?[Y/n] " install
    if [ "$install" = "n" ]; then
      echo "bye"
      exit 0
    fi
  fi
  if [ "$(uname)" = "Darwin" ]; then
    echo "Installing kubectl on Darwin"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/darwin/amd64/kubectl
  elif [ "$(expr substr $(uname -s) 1 5)" = "Linux" ]; then
    echo "Installing kubectl on Linux"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
  else 
    echo "No support to that system"
    exit 1
  fi

  chmod +x ./kubectl
  sudo mv ./kubectl /usr/local/bin/kubectl
fi

if [ ! $(which minikube) ]; then
  if [ -z "$MAESTRO_TEST_CI" ]; then
    read -p "Minikube not installed. Wish to install and proceed with integration tests?[Y/n] " install
    if [ "$install" = "n" ]; then
      echo "bye"
      exit 0
    fi
  fi
  if [ "$(uname)" = "Darwin" ]; then
    echo Installing minikube on Darwin
    brew update
    brew cask install minikube
    brew install --HEAD xhyve
  elif [ "$(expr substr $(uname -s) 1 5)" = "Linux" ]; then
    echo Installing KVM
    curl -L https://github.com/dhiltgen/docker-machine-kvm/releases/download/v0.10.0/docker-machine-driver-kvm-ubuntu14.04 > docker-machine-driver-kvm
    chmod +x docker-machine-driver-kvm
    sudo mv docker-machine-driver-kvm /usr/local/bin/docker-machine-driver-kvm
    echo Installing helpers
    sudo apt-get install libvirt-bin qemu-kvm
    echo Installing minikube on Linux
    curl -Lo minikube https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
    chmod +x minikube 
    sudo mv minikube /usr/local/bin/
  else 
    echo No support to that system
    exit 1
  fi
fi 
 
localKubeVersion=$(kubectl version --short=true | awk '/Server/{print $3}')
desiredKubeVersion=$(cat ./metadata/version.go | grep "KubeVersion" | egrep -oh "v(\d+\.?)+")
if [ "$localKubeVersion" != "$desiredKubeVersion" ]; then
  echo "Your kubernetes version is $localKubeVersion, please update to the latest version before running Integration Tests"
  #exit 1
fi

echo Starting minikube
if [ $(minikube ip) ]; then
  echo Minikube already started
elif [ "$(uname)" = "Darwin" ]; then
  minikube start --vm-driver=xhyve --kubernetes-version $desiredKubeVersion
elif [ "$(expr substr $(uname -s) 1 5)" = "Linux" ]; then
  sudo minikube start --vm-driver=kvm
else
    echo No support to that system
    exit 1
fi

echo "Getting minikube host"
echo `kubectl config view | grep $(minikube ip) | sed -e "s/ *server: \(.*\)/\1/"` > /tmp/MAESTRO_MINIKUBE_HOST
