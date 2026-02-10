# SOP: Deployment

## Overview
This document describes the deployment procedures for StreamMaxing to AWS infrastructure (Lambda + S3 + CloudFront + API Gateway).

---

## Quick Deploy (Recommended)

The project includes a SAM template and PowerShell deployment script that automates the entire process.

### Prerequisites
- **AWS CLI**: `aws --version` (configured with `aws configure`)
- **AWS SAM CLI**: `sam --version` (install: https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- **Go 1.22+**: `go version`
- **Node.js 18+**: `node --version`

### One-Command Deploy

```powershell
# Full deploy (backend + frontend)
.\deploy.ps1

# Backend only
.\deploy.ps1 -BackendOnly

# Frontend only (after backend is already deployed)
.\deploy.ps1 -FrontendOnly
```

### What the Script Does
1. Reads secrets from `.env` file
2. Cross-compiles Go backend for Linux x86_64
3. Deploys SAM template (Lambda + API Gateway + S3 + CloudFront)
4. Builds the React frontend
5. Uploads frontend to S3
6. Invalidates CloudFront cache

### Architecture
CloudFront acts as a reverse proxy with two origins:
- `/api/*` and `/webhooks/*` → API Gateway → Lambda (backend)
- Everything else → S3 (React frontend)

This means the same domain serves both frontend and API, eliminating CORS and cross-domain cookie issues.

### After First Deploy
1. Copy the CloudFront URL from the deployment output
2. Update Discord Developer Portal → OAuth2 → Redirects: `https://<cloudfront-url>/api/auth/discord/callback`
3. Update Twitch Developer Console → OAuth → Redirect URLs: `https://<cloudfront-url>/api/auth/twitch/callback`
4. Re-link any streamers (to create EventSub subscriptions with the new HTTPS URL)

### Adding Custom Domain (streammaxing.live)
See the "Set Up Custom Domain" section below for ACM certificate + CloudFront alias + DNS configuration.

---

## Manual Deployment (Reference)

The sections below document manual AWS CLI commands for each resource. The `deploy.ps1` script automates all of this, but these commands are useful for debugging or one-off changes.

## Prerequisites

### Required Tools
- **AWS CLI**: `aws --version`
- **AWS Account**: With appropriate permissions
- **Go 1.22+**: For building Lambda function
- **Node.js 18+**: For building frontend

### AWS Services Used
- **Lambda**: Backend API
- **API Gateway**: HTTP API routing
- **S3**: Frontend hosting
- **CloudFront**: CDN for frontend
- **Neon**: PostgreSQL database (external)
- **CloudWatch**: Logs and monitoring

---

## Initial Infrastructure Setup

### 1. Create S3 Bucket for Frontend

```bash
# Create bucket
aws s3 mb s3://streammaxing-frontend --region us-east-1

# Enable static website hosting
aws s3 website s3://streammaxing-frontend \
  --index-document index.html \
  --error-document index.html
```

**Bucket Policy** (for CloudFront OAC):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowCloudFrontServicePrincipalReadOnly",
      "Effect": "Allow",
      "Principal": {
        "Service": "cloudfront.amazonaws.com"
      },
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::streammaxing-frontend/*",
      "Condition": {
        "StringEquals": {
          "AWS:SourceArn": "arn:aws:cloudfront::ACCOUNT_ID:distribution/DISTRIBUTION_ID"
        }
      }
    }
  ]
}
```

---

### 2. Create CloudFront Distribution

```bash
aws cloudfront create-distribution \
  --origin-domain-name streammaxing-frontend.s3.us-east-1.amazonaws.com \
  --default-root-object index.html
```

**Key Configuration**:
- **Origin Access**: Use Origin Access Control (OAC)
- **Viewer Protocol Policy**: Redirect HTTP to HTTPS
- **Caching**:
  - `/assets/*`: Cache for 1 year
  - `/index.html`: No cache
- **Custom Error Responses**:
  - 403 → `/index.html` (for SPA routing)
  - 404 → `/index.html` (for SPA routing)

**Get Distribution ID**:
```bash
aws cloudfront list-distributions \
  --query "DistributionList.Items[?Comment=='StreamMaxing'].Id" \
  --output text
```

---

### 3. Create Lambda Function

#### Create Execution Role

**File**: `trust-policy.json`

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

```bash
# Create role
aws iam create-role \
  --role-name StreamMaxingLambdaRole \
  --assume-role-policy-document file://trust-policy.json

# Attach policies
aws iam attach-role-policy \
  --role-name StreamMaxingLambdaRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
```

#### Create Lambda Function

```bash
# Create placeholder function first
echo 'exports.handler = async (event) => ({ statusCode: 200 });' > index.js
zip function.zip index.js

aws lambda create-function \
  --function-name streammaxing \
  --runtime provided.al2 \
  --role arn:aws:iam::ACCOUNT_ID:role/StreamMaxingLambdaRole \
  --handler bootstrap \
  --zip-file fileb://function.zip \
  --timeout 30 \
  --memory-size 256

# Clean up placeholder
rm index.js function.zip
```

#### Set Environment Variables

```bash
aws lambda update-function-configuration \
  --function-name streammaxing \
  --environment "Variables={
    DATABASE_URL=postgresql://user:pass@neon.tech/db,
    DISCORD_CLIENT_ID=xxx,
    DISCORD_CLIENT_SECRET=xxx,
    DISCORD_BOT_TOKEN=xxx,
    TWITCH_CLIENT_ID=xxx,
    TWITCH_CLIENT_SECRET=xxx,
    TWITCH_WEBHOOK_SECRET=xxx,
    JWT_SECRET=xxx,
    API_BASE_URL=https://api.streammaxing.com,
    FRONTEND_URL=https://streammaxing.com,
    ENVIRONMENT=production
  }"
```

**Note**: For sensitive values, use AWS Secrets Manager (see below).

---

### 4. Create API Gateway

```bash
# Create HTTP API
aws apigatewayv2 create-api \
  --name streammaxing-api \
  --protocol-type HTTP \
  --target arn:aws:lambda:us-east-1:ACCOUNT_ID:function:streammaxing

# Get API ID
API_ID=$(aws apigatewayv2 get-apis \
  --query "Items[?Name=='streammaxing-api'].ApiId" \
  --output text)

# Create default stage
aws apigatewayv2 create-stage \
  --api-id $API_ID \
  --stage-name '$default' \
  --auto-deploy
```

**Get API Endpoint**:
```bash
aws apigatewayv2 get-apis \
  --query "Items[?Name=='streammaxing-api'].ApiEndpoint" \
  --output text
```

Example: `https://abc123.execute-api.us-east-1.amazonaws.com`

#### Configure CORS

```bash
aws apigatewayv2 update-api \
  --api-id $API_ID \
  --cors-configuration "AllowOrigins=https://your-cloudfront-url.cloudfront.net,AllowMethods=GET,POST,PUT,DELETE,OPTIONS,AllowHeaders=Content-Type,Authorization,AllowCredentials=true"
```

---

### 5. Set Up Custom Domain (Optional)

#### Request SSL Certificate (ACM)

```bash
# Certificate must be in us-east-1 for CloudFront
aws acm request-certificate \
  --domain-name streammaxing.com \
  --subject-alternative-names www.streammaxing.com api.streammaxing.com \
  --validation-method DNS \
  --region us-east-1
```

**Verify Certificate**: Add CNAME records to DNS

#### Configure CloudFront with Custom Domain

```bash
aws cloudfront update-distribution \
  --id DISTRIBUTION_ID \
  --aliases streammaxing.com,www.streammaxing.com \
  --viewer-certificate ACMCertificateArn=arn:aws:acm:...,SSLSupportMethod=sni-only
```

#### Configure API Gateway with Custom Domain

```bash
aws apigatewayv2 create-domain-name \
  --domain-name api.streammaxing.com \
  --domain-name-configurations CertificateArn=arn:aws:acm:...

aws apigatewayv2 create-api-mapping \
  --domain-name api.streammaxing.com \
  --api-id $API_ID \
  --stage '$default'
```

#### Update DNS Records

```
streammaxing.com       A      ALIAS cloudfront-distribution-domain
www.streammaxing.com   A      ALIAS cloudfront-distribution-domain
api.streammaxing.com   A      ALIAS api-gateway-domain
```

---

## Deployment Procedures

### Backend Deployment

#### 1. Build Lambda Binary

```bash
cd backend

# Build for Lambda (Linux x86_64)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap cmd/lambda/main.go

# Create deployment package
zip deployment.zip bootstrap

# Clean up
rm bootstrap
```

#### 2. Deploy to Lambda

```bash
aws lambda update-function-code \
  --function-name streammaxing \
  --zip-file fileb://deployment.zip

# Wait for update to complete
aws lambda wait function-updated \
  --function-name streammaxing

# Test function
aws lambda invoke \
  --function-name streammaxing \
  --payload '{"requestContext":{"http":{"method":"GET","path":"/api/health"}}}' \
  response.json

cat response.json
```

#### 3. Verify Deployment

```bash
# Check function status
aws lambda get-function \
  --function-name streammaxing \
  --query 'Configuration.[State,LastUpdateStatus]'

# View logs
aws logs tail /aws/lambda/streammaxing --follow
```

---

### Frontend Deployment

#### 1. Build Frontend

```bash
cd frontend

# Set production API URL
echo "VITE_API_URL=https://api.streammaxing.com" > .env.production

# Build
npm run build
```

**Verify Build**:
```bash
ls -lh dist/
# Should see index.html, assets/, etc.
```

#### 2. Deploy to S3

```bash
# Sync build to S3 (delete removed files)
aws s3 sync dist/ s3://streammaxing-frontend --delete

# Set cache headers
aws s3 cp s3://streammaxing-frontend/index.html s3://streammaxing-frontend/index.html \
  --metadata-directive REPLACE \
  --cache-control "no-cache" \
  --content-type "text/html"

aws s3 cp s3://streammaxing-frontend/assets/ s3://streammaxing-frontend/assets/ \
  --recursive \
  --metadata-directive REPLACE \
  --cache-control "max-age=31536000,immutable"
```

#### 3. Invalidate CloudFront Cache

```bash
# Get distribution ID
DISTRIBUTION_ID=$(aws cloudfront list-distributions \
  --query "DistributionList.Items[?Comment=='StreamMaxing'].Id" \
  --output text)

# Create invalidation
aws cloudfront create-invalidation \
  --distribution-id $DISTRIBUTION_ID \
  --paths "/*"

# Wait for invalidation to complete
aws cloudfront wait invalidation-completed \
  --distribution-id $DISTRIBUTION_ID \
  --id INVALIDATION_ID
```

#### 4. Verify Deployment

```bash
# Test frontend
curl -I https://your-cloudfront-url.cloudfront.net

# Expected: 200 OK

# Test asset caching
curl -I https://your-cloudfront-url.cloudfront.net/assets/index-abc123.js

# Expected: Cache-Control: max-age=31536000
```

---

## Database Migrations

### Apply Migration to Production

```bash
# Set production DATABASE_URL
export DATABASE_URL="postgresql://user:pass@prod.neon.tech/db"

# Apply migration
psql $DATABASE_URL -f migrations/00X_new_migration.sql

# Verify
psql $DATABASE_URL -c "\dt"
```

---

## Rollback Procedures

### Rollback Backend

#### Option 1: Redeploy Previous Version

```bash
# List previous versions
aws lambda list-versions-by-function \
  --function-name streammaxing \
  --query 'Versions[*].[Version,LastModified]'

# Update alias to point to previous version
aws lambda update-alias \
  --function-name streammaxing \
  --name production \
  --function-version 5  # Previous working version
```

#### Option 2: Redeploy from Git

```bash
# Checkout previous commit
git checkout PREVIOUS_COMMIT_HASH

# Build and deploy
GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/lambda/main.go
zip deployment.zip bootstrap
aws lambda update-function-code --function-name streammaxing --zip-file fileb://deployment.zip

# Return to main branch
git checkout main
```

---

### Rollback Frontend

```bash
# Checkout previous version
git checkout PREVIOUS_COMMIT_HASH

# Build and deploy
cd frontend
npm run build
aws s3 sync dist/ s3://streammaxing-frontend --delete
aws cloudfront create-invalidation --distribution-id $DISTRIBUTION_ID --paths "/*"

# Return to main branch
git checkout main
```

---

### Rollback Database

```bash
# Apply rollback migration
psql $DATABASE_URL -f migrations/00X_new_migration.down.sql
```

---

## Monitoring

### CloudWatch Logs

#### View Backend Logs

```bash
# Tail logs in real-time
aws logs tail /aws/lambda/streammaxing --follow

# View recent errors
aws logs filter-log-events \
  --log-group-name /aws/lambda/streammaxing \
  --filter-pattern "ERROR" \
  --start-time $(date -u -d '1 hour ago' +%s)000
```

#### View API Gateway Logs

```bash
# Enable logging first
aws apigatewayv2 update-stage \
  --api-id $API_ID \
  --stage-name '$default' \
  --access-log-settings '{"DestinationArn":"arn:aws:logs:...","Format":"$context.requestId"}'

# View logs
aws logs tail /aws/apigateway/streammaxing-api --follow
```

---

### CloudWatch Metrics

#### Lambda Metrics

```bash
# Invocations
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Invocations \
  --dimensions Name=FunctionName,Value=streammaxing \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum

# Errors
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Errors \
  --dimensions Name=FunctionName,Value=streammaxing \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum
```

---

### CloudWatch Alarms

#### Lambda Error Rate Alarm

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-LambdaErrors" \
  --alarm-description "Alert when Lambda error rate > 5%" \
  --metric-name Errors \
  --namespace AWS/Lambda \
  --statistic Average \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 0.05 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=FunctionName,Value=streammaxing
```

#### Billing Alarm

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-BillingAlert" \
  --alarm-description "Alert when bill exceeds $20" \
  --metric-name EstimatedCharges \
  --namespace AWS/Billing \
  --statistic Maximum \
  --period 21600 \
  --evaluation-periods 1 \
  --threshold 20 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=Currency,Value=USD
```

---

## Secrets Management

### Using AWS Secrets Manager

#### Store Secret

```bash
aws secretsmanager create-secret \
  --name streammaxing/production/discord \
  --secret-string '{"client_id":"xxx","client_secret":"xxx","bot_token":"xxx"}'

aws secretsmanager create-secret \
  --name streammaxing/production/twitch \
  --secret-string '{"client_id":"xxx","client_secret":"xxx","webhook_secret":"xxx"}'
```

#### Grant Lambda Access

```bash
aws iam attach-role-policy \
  --role-name StreamMaxingLambdaRole \
  --policy-arn arn:aws:iam::aws:policy/SecretsManagerReadWrite
```

#### Access in Code

```go
import (
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
)

func getSecret(secretName string) (string, error) {
    sess := session.Must(session.NewSession())
    svc := secretsmanager.New(sess)

    result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretName),
    })
    if err != nil {
        return "", err
    }

    return *result.SecretString, nil
}

// Usage
discordSecrets, _ := getSecret("streammaxing/production/discord")
```

---

## Automated Deployment

### GitHub Actions

**File**: `.github/workflows/deploy.yml`

```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Build Lambda
        run: |
          cd backend
          GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap cmd/lambda/main.go
          zip deployment.zip bootstrap

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1

      - name: Deploy to Lambda
        run: |
          aws lambda update-function-code \
            --function-name streammaxing \
            --zip-file fileb://backend/deployment.zip

  deploy-frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Node
        uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Build Frontend
        run: |
          cd frontend
          npm install
          npm run build
        env:
          VITE_API_URL: https://api.streammaxing.com

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1

      - name: Deploy to S3
        run: |
          aws s3 sync frontend/dist/ s3://streammaxing-frontend --delete

      - name: Invalidate CloudFront
        run: |
          aws cloudfront create-invalidation \
            --distribution-id ${{ secrets.CLOUDFRONT_DISTRIBUTION_ID }} \
            --paths "/*"
```

---

## Health Checks

### Backend Health Check

```bash
curl https://api.streammaxing.com/api/health
```

Expected response:
```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:00:00Z"
}
```

---

### Frontend Health Check

```bash
curl -I https://streammaxing.com
```

Expected: `200 OK`

---

### Database Health Check

```bash
psql $DATABASE_URL -c "SELECT 1"
```

Expected: `1`

---

## Troubleshooting

### Issue: Lambda timeout

**Solution**: Increase timeout
```bash
aws lambda update-function-configuration \
  --function-name streammaxing \
  --timeout 60
```

---

### Issue: Out of memory

**Solution**: Increase memory
```bash
aws lambda update-function-configuration \
  --function-name streammaxing \
  --memory-size 512
```

---

### Issue: CloudFront serving old version

**Solution**: Invalidate cache
```bash
aws cloudfront create-invalidation \
  --distribution-id $DISTRIBUTION_ID \
  --paths "/*"
```

---

### Issue: API Gateway 502 Error

**Possible Causes**:
- Lambda function crashed
- Lambda timeout
- Lambda function not returning proper response

**Solution**: Check CloudWatch Logs
```bash
aws logs tail /aws/lambda/streammaxing --follow
```

---

## Checklist

- [ ] S3 bucket created
- [ ] CloudFront distribution created
- [ ] Lambda function created
- [ ] API Gateway created
- [ ] IAM roles configured
- [ ] Environment variables set
- [ ] Custom domain configured (optional)
- [ ] Backend deployed successfully
- [ ] Frontend deployed successfully
- [ ] Database migrations applied
- [ ] Health checks passing
- [ ] CloudWatch alarms configured
- [ ] Secrets stored securely
- [ ] CI/CD pipeline configured (optional)
- [ ] Rollback procedure tested

---

## Cost Optimization

- Use Lambda reserved concurrency to prevent runaway costs
- Enable CloudFront caching to reduce Lambda invocations
- Use Neon's free tier (0.5GB storage, 100 hours compute/month)
- Set CloudWatch log retention to 7 days
- Use S3 Intelligent-Tiering for frontend assets
