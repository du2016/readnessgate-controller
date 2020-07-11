# readnessgate-controller

use label to control readnessgate

# test

- expose the pod to create a endpoint

```
kubectl expose pod centos --port=80 --target-port=80
```

- label pods to change condition

```
kubectl label pod centos example="true" --overwrite
kubectl label pod centos example="false" --overwrite
```

- get the condition of pod

```
kubectl get pods centos -o json | jq .status.conditions
```