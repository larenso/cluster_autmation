kubectl apply -k .
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
helm install cilium oci://quay.io/cilium/charts/cilium --version 1.20.5 -n cilium -f cilium/vals.yml -f cilium/vals-mon.yml
sleep 90
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager --version v1.20.5 -n cert-manager -f cert-manager/vals.yml
sleep 90
helm install porkbun buffchart -n cert-manager -f cert-manager/porkbun.yml --set image.tag=0.8-alpine3.24
kubectl apply -k cert-manager
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack --version 93.10.0 -n monitoring -f monitoring/vals.yml --set prometheusOperator.admissionWebhooks.enabled=false
sleep 60
helm upgrade kube-prometheus-stack prometheus-community/kube-prometheus-stack --version 93.10.0 -n monitoring -f monitoring/vals.yml
sleep 60
kubectl apply -k cilium
kubectl apply -k monitoring -n monitoring
kubectl apply -k ha
kubectl apply -k immich
helm install ha buffchart -n apps -f ha/vals.yml --set image.tag=2026.11.3
helm install vchordpostgres buffchart -n apps -f immich/vchordpostgres.yml --set image.tag=pg19.0-v1.5.0
helm install valkey buffchart -n apps -f immich/valkey.yml --set image.tag=9.2.0 --set sidecars.redis-exporter.tag=v1.99.0-alpine
helm install ml buffchart -n apps -f immich/ml.yml --set image.tag=v3.4.2
helm install immich buffchart -n apps -f immich/immich.yml --set image.tag=v3.4.2
