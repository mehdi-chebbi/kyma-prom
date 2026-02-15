@echo off
echo Loading environment variables...
nerdctl build -t ldap-manager . 
nerdctl save -o ldap-manager.tar ldap-manager
echo saved clear
nerdctl load --namespace k8s.io -i ldap-manager.tar
kubectl delete deployment ldap-manager -n dev-platform
kubectl apply -f .\k8s\03-ldap-manager.yaml
kubectl get deployment ldap-manager -n dev-platform