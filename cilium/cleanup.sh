ip link delete cilium_host
ip link delete cilium_net
ip link delete cilium_wg0
rm -r /sys/fs/bpf/cilium
rm -r /sys/fs/bpf/tc
rm -r /mnt/data/k3storage
./cilium  status --kubeconfig /etc/rancher/k3s/k3s.yaml
