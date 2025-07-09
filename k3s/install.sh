rm -rf /etc/rancher/node
mkdir /etc/rancher/k3s

cp /etc/rancher/config.yaml /etc/rancher/k3s/config.yaml

curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.34.1+k3s1 sh -
