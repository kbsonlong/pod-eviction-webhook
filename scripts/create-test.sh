#!/bin/bash

contorllerNode=$(kubectl get pod -l app=pod-eviction-protection -o jsonpath="{.items[].spec.nodeName}")

testNodes=$(kubectl get node -o jsonpath="{.items[*].metadata.name}" | sed "s/${contorllerNode}//g;s/webhook-test-control-plane//g")

kubectl create ns webhook-test
kubectl apply -f deploy/test-deploy.yaml
kubectl rollout status -n webhook-test deploy nginx-deployment

# stop kubelet
for node in ${testNodes}
do
  docker exec ${node} systemctl stop kubelet
done