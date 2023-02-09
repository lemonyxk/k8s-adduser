### make k8s add user easier to use

```shell
# 1.create user with server ca.
$ k8s-adduser -u username -ca client-ca.crt -caKey client-ca.key -sca server-ca.crt -url https://127.0.0.1:6443

# 2.create user with kubeconfig.
$ k8s-adduser -u username -ca client-ca.crt -caKey client-ca.key -kubeconfig ~/.kube/config
```