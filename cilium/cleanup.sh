ip link delete cilium_host
ip link delete cilium_net
ip link delete cilium_vxlan
ip link delete cilium_wg0
/sbin/iptables-save | grep -iv cilium | /sbin/iptables-restore
/sbin/ip6tables-save | grep -iv cilium | /sbin/ip6tables-restore
rm -r /sys/fs/bpf/cilium
rm -r /sys/fs/bpf/tc
rm -r /mnt/data/k3storage
./cilium  status --kubeconfig /etc/rancher/k3s/k3s.yaml
