import AWS from '@aws-sdk/client-s3'
import express from 'express'
import sts from "@aws-sdk/client-sts"
import { fromTemporaryCredentials } from "@aws-sdk/credential-providers"

const app = express()
const port = 30000
// var stsclient = new sts.STS()

// assumeRoleResult = stsclient.assumeRole(role-arn, function (err, data){
//     if (err) console.log(err, err.stack); // an error occurred
//     else     console.log(data); 
// });
// tempCredentials = new SessionAWSCredentials(
//    assumeRoleResult.AccessKeyId, 
//    assumeRoleResult.SecretAccessKey, 
//    assumeRoleResult.SessionToken);

//"http://localstack.k8s-s3-operator-system:4566"
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










  app.listen(port,()=>{
    console.log(`test app listening on port ${port}`)
  })