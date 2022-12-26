import AWS from '@aws-sdk/client-s3'
import express from 'express'
import { fromTemporaryCredentials } from "@aws-sdk/credential-providers"
import k8s from '@kubernetes/client-node'
import util from 'util'


const app = express()
const port = 30000
const NAMESPACE = process.env.NAMESPACE || "k8s-s3-operator-system"
const SERVICE_ACCOUNT_NAME = process.env.SERVICE_ACCOUNT_NAME || "k8s-s3-operator-controller-manager"
const PREFIX_NAME = "system:serviceaccounts"
const PREFIX_UNAME = "system:serviceaccount"
const AUTH_GROUP = "system:authenticated"
const GROUP_TO_VALID = [PREFIX_NAME,PREFIX_NAME+':'+NAMESPACE, AUTH_GROUP]
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
kc.loadFromDefault()
const authk8sApi = kc.makeApiClient(k8s.AuthenticationV1Api);
const coreK8sApi =  kc.makeApiClient(k8s.CoreV1Api)





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
        console.log(`got token ${token}`)
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
    authk8sApi.createTokenReview(body)
   .then(async (k8sRes)=>{
    if (k8sRes.body.status.error){
        res.status(403).send(k8sRes.body)
    }else{
        const isvalid =await validateResFromTokenReview(k8sRes.body)
        if (isvalid[0]){
            res.status(200).send(k8sRes.body)
        }
        else{
            res.status(403).send(isvalid[1])
        }

    }
    
   })
   .catch ((err)=> {
    console.error(` catch error ${err}`)
    res.status(500).send("error in AC")

   })
})


  app.listen(port,()=>{
    console.log(`test app listening on port ${port}`)
  })

  async function validateResFromTokenReview(res){
    var msg = "is valid"
    const resStatus = res.status
    console.log(`res to valid is ${util.inspect( resStatus.user, {depth: null})}`)
    if (!(validateGroups(resStatus.user.groups,GROUP_TO_VALID))){
        return ([false, "groups not valid"])
        
    }
    if (!(validateUserName(resStatus.user.username))){
        return ([false,"user name is not valid"])
    }
    if (!(await validateUid(resStatus.user.uid))){
        return ([false, "uid is not valid"])
    }
    return [true,msg]

  }
  function validateGroups(groups,groupsToValid){
    console.log(`validateGroups group1 : ${groups},\n group2: ${groupsToValid}`)
    try{
    if(groups.length === groupsToValid.length){
        return groups.every(element =>{
            if (groupsToValid.includes(element)){
                return true
            }
            console.log(element)
            return false
        })
    }}
    catch{
        console.log("catch err")
        return false
    }
    return false

  }
  function validateUserName(username){
    console.log(`validateUserName ${username}`)
    const expectUser = PREFIX_UNAME + ':' + NAMESPACE + ':' + SERVICE_ACCOUNT_NAME
    console.log(expectUser)
    return username === expectUser

  }
  async function validateUid(uid){
    console.log(`validateUid function got uid: ${uid}`)

    try {
          const SA = await coreK8sApi.readNamespacedServiceAccount(SERVICE_ACCOUNT_NAME, NAMESPACE)
          console.log(`got service account ${SA}`)
          return SA.body.metadata.uid === uid
      } catch (err) {
          console.log(`error in validateUid ${err}`)
          return false
      }


   }
