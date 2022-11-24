package aws

type ResourceMap struct {
	resources map[string]string
}

func(rMap *ResourceMap) AddResource(k8sResourceName string, bucketName string){
	rMap.resources[k8sResourceName] = bucketName
 }
 func(rMap *ResourceMap)GetBucketNameForResource(k8sResourceName string)string{
	return rMap.resources[k8sResourceName]
 }
 func(rMap *ResourceMap) RemoveResource(k8sResourceName string){
	delete(rMap.resources,k8sResourceName)
 }
