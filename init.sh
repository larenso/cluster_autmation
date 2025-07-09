kubectl apply -k .
helm repo add cilium https://helm.cilium.io/ --force-update
helm repo add argo https://argoproj.github.io/argo-helm --force-update
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
helm repo add jetstack https://charts.jetstack.io --force-update

helm install cilium cilium/cilium --version 1.18.2 -n cilium -f cilium/vals.yml
helm install kube-prometheus-crds prometheus-community/kube-prometheus-stack --version 78.3.2 -n monitoring -f monitoring/crds.yml
helm uninstall kube-prometheus-crds -n monitoring
helm install cert-manager jetstack/cert-manager --version v1.19.1 -n cert-manager -f cert-manager/vals.yml
helm install valkey buffchart -n valkey -f valkey/vals.yml
helm install argocd argo/argo-cd --version 9.0.1 -n argocd -f argocd/vals.yml
