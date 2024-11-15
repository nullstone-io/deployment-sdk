# deployment-sdk

This is an open-source tool to perform various application actions in a platform-agnostic manner.

## Support for app types

- Containers
  - AWS ECS (Fargate)
  - AWS ECS (EC2)
  - AWS Batch (Fargate)
  - GCP GKE
- Serverless
  - AWS Lambda (Zip file)
  - AWS Lambda (Docker image)
- Static Sites
  - AWS S3
- VM
  - AWS Elastic Beanstalk

## Actions

Each implementation in this library implements a set of actions depending on the app type:
- Publish artifact
- Execute deployment
- Log and monitor deployment
- Stream application logs
