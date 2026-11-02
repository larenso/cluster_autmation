kubectl apply -k .
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
sleep 5
helm install kube-prometheus-crds prometheus-community/kube-prometheus-stack --version 92.11.0 -n monitoring -f monitoring/crds.yml
sleep 5
helm install cilium oci://quay.io/cilium/charts/cilium --version 1.20.4 -n cilium -f cilium/vals.yml -f cilium/vals-mon.yml
sleep 90
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager --version v1.20.5 -n cert-manager -f cert-manager/vals.yml
sleep 60
kubectl apply -k cert-manager
kubectl apply -k monitoring -n monitoring --server-side
helm install porkbun buffchart -n cert-manager -f cert-manager/porkbun.yml --set image.tag=0.7-alpine3.24
sleep 5
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack --version 92.16.2 -n monitoring -f monitoring/vals.yml --set prometheusOperator.admissionWebhooks.enabled=false
sleep 60
helm upgrade kube-prometheus-stack prometheus-community/kube-prometheus-stack --version 92.16.2 -n monitoring -f monitoring/vals.yml
sleep 60
kubectl apply -k cilium
kubectl apply -k ha
kubectl apply -k immich
helm install ha buffchart -n apps -f ha/vals.yml --set image.tag=2026.10.4
helm install postgres buffchart -n apps -f immich/postgres.yml --set image.tag=pg17-v1.1.1-arm64
helm install valkey buffchart -n apps -f immich/valkey.yml --set image.tag=9.1.3 --set sidecars.redis-exporter.tag=v1.96.0-alpine
helm install ml buffchart -n apps -f immich/ml.yml --set image.tag=v3.3.5
helm install immich buffchart -n apps -f immich/immich.yml --set image.tag=v3.3.5
