import AWS from '@aws-sdk/client-s3'
import express from 'express'
import { fromTemporaryCredentials } from "@aws-sdk/credential-providers"
import k8s from '@kubernetes/client-node'


const app = express()
const port = 30000
const NAMESPACE = "k8s-s3-operator-system"
const SERVICE_ACCOUNT_NAME = "k8s-s3-operator-controller-manager"
const PREFIX_NAME = "system:serviceaccounts"
const creds =  await fromTemporaryCredentials({params: {RoleArn: 'arn:aws:iam:::role/s3bucket-sample-app-testtIAM-ROLE-S3Operator'},
                                                clientConfig: {region: 'eu-central-1'},
                                                endpoint:"http://localstack.k8s-s3-operator-system:4566"})
var s3 = new AWS.S3({
    endpoint: "http://localstack.k8s-s3-operator-system:4566" ,
    region:'eu-central-1',
    s3ForcePathStyle: true,
    sslEnabled: true ,
    apiVersion: '2006-03-01',
    credentials: creds
    
})
const kc = new k8s.KubeConfig();
kc.loadFromCluster()
console.log(kc.getCurrentCluster())
const k8sApi = kc.makeApiClient(k8s.AuthenticationV1Api);





app.get('/', (req, res) => {
    res.send('app test service is up')
  })

app.get('/bucket/:bucket_name', async(req,res) =>{
    console.log(`get bucket, bucket name - ${req.params.bucket_name}`)
    console.log(await creds.toString)

    const params = {Bucket: req.params.bucket_name}
    console.log(`s3 creds ${s3.config.credentials}`)
    s3.getBucketLocation(params,(err,data)=>{
        if (err){
            console.log(err,err.stack)
            res.status(err.statusCode).send(err.code)
        }else{
            res.status(200).send(data)
        }
    })
})

app.get('/bucket/:bucket_name/:obj_id', (req,res)=>{
    console.log(`get obj, bucket name - ${req.params.bucket_name} , obj id- ${req.params.obj_id} `)
    const params = {Bucket: req.params.bucket_name,Key: req.params.obj_id}
    s3.getObject(params,(err,data)=>{
        if (err){
            console.log(err,err.stack)
            res.status(500).send("error to get obj")
        }else{
            res.status(200).send(data)
        }
    })
})
app.post('/bucket/:bucket_name', (req,res)=>{
    console.log(`post obj, bucket name - ${req.params.bucket_name} , obj  ${req.body} `)
    const params = {Bucket: req.params.bucket_name,Key: req.body.Key, Body:JSON.stringify(req.body.Body)}
    s3.putObject(params,(err,data)=>{
        if (err){
            console.log(err,err.stack)
            res.status(500).send("error to get obj")
        }else{
            res.status(200).send(data)
        }
    })
})

app.post('/',async (req,res)  =>{
    console.log("create service account function")
    var token
    try{
        token = req.headers.token
    }catch(e){
        console.log("no token in header")
        res.status(400).send("bad request")
    }
    const body = {
        apiVersion: 'authentication.k8s.io/v1',
        kind: 'TokenReview',
        spec: {
          token: req.headers.token
        }
      };
    try{
   await k8sApi.createTokenReview(body).then((k8sRes)=>{
    console.log(`response from k8s api server ${k8sRes.body}`)
    if (k8sRes.body.status.error){
        res.status(403).send(k8sRes.body)
    }else{
        res.status(200).send(k8sRes.body)
    }
    
   })}
   catch (e){
    console.log(`catch error ${e}`)
    res.status(500).send("error in AC")

   }

})


  app.listen(port,()=>{
    console.log(`test app listening on port ${port}`)
  })
  function validateResFromTokenReview(res){
    var msg = "is valid"
    validateGroups(res.status.groups,res.status.groups)
    validateUserName(res.status.username)
    validateUid(res.status.uid)
    return [true,msg]

  }
  function validateGroups(groups,groupsToValid){
    if(groups.length === groupsToValid){
        return groups.every(element =>{
            if (groupsToValid.includes(element)){
                return true
            }
            return false
        })
    }
    return false

  }
  function validateUserName(username){
    return username === PREFIX_NAME + ':' + NAMESPACE + ':' + SERVICE_ACCOUNT_NAME

  }
  function validateUid(uid){

  }