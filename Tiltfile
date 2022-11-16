docker_build('controller:local', '.', 
    dockerfile='./Dockerfile')
k8s_yaml('./config/manager/manager.yaml')
k8s_resource('controller-manager', port_forwards=8000)