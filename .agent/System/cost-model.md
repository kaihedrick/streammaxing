# Cost Model & Monitoring

## Overview

Target monthly cost: **Less than $20/month** for multi-server support with reasonable usage (10-50 streamers, 5-10 Discord servers, 1,000-5,000 notifications/month).

---

## Cost Breakdown

### Neon Postgres

**Free Tier**:
- 5 GB storage
- 0.5 compute units (shared vCPU)
- 100 hours of compute per month
- **Cost**: $0

**Starter Tier** (if free tier exceeded):
- 10 GB storage
- 1 compute unit (0.25 vCPU)
- Unlimited compute hours
- **Cost**: ~$5/month

**Expected Usage**:
- Database size: ~50 MB (1,000 users, 100 streamers, 10 guilds)
- Compute: ~10 hours/month (serverless, scales to zero)
- **Estimated Cost**: $0 (free tier sufficient)

**Scaling Threshold**:
- Stay in free tier up to ~10,000 users or 500 streamers
- Move to Starter tier if compute hours exceed 100/month

---

### AWS Lambda

**Free Tier** (permanent):
- 1 million requests/month
- 400,000 GB-seconds of compute/month

**Pricing** (after free tier):
- $0.20 per 1 million requests
- $0.0000166667 per GB-second

**Expected Usage**:
- **Invocations**: ~10,000/month (API calls + webhooks)
  - 1,000 webhook events (stream.online)
  - 5,000 API requests (dashboard, preferences)
  - 4,000 OAuth callbacks and misc
- **Compute**: 256 MB, 500ms average execution = 128 GB-seconds/month
- **Cost**: $0 (well within free tier)

**Scaling Threshold**:
- Free tier covers up to 1M requests/month
- At 100,000 requests/month: ~$20/month (after free tier)
- **Mitigation**: Optimize cold starts, cache API responses

---

### API Gateway (HTTP API)

**Free Tier** (12 months):
- 1 million requests/month

**Pricing** (after free tier):
- $1.00 per million requests (first 300M requests)

**Expected Usage**:
- ~10,000 requests/month
- **Year 1 Cost**: $0 (free tier)
- **Year 2+ Cost**: $0.01/month (negligible)

**Scaling Threshold**:
- At 1M requests/month: $1/month
- At 10M requests/month: $10/month (would be a success problem)

---

### S3 Storage

**Pricing**:
- $0.023 per GB/month (Standard storage)
- $0.09 per GB transferred out (first 10 TB)

**Expected Usage**:
- Storage: ~100 MB (React build)
- Data transfer: Minimal (CloudFront caching reduces S3 egress)
- **Cost**: ~$0.23/month

**Note**: Data transfer is almost entirely through CloudFront, not direct S3 egress

---

### CloudFront

**Free Tier** (12 months):
- 1 TB data transfer out/month
- 10 million HTTP/HTTPS requests/month

**Pricing** (after free tier):
- $0.085 per GB for first 10 TB (data transfer out)
- $0.0075 per 10,000 HTTPS requests

**Expected Usage**:
- Data transfer: ~50 GB/month (100 users, ~500 MB/user)
- Requests: ~50,000/month (API + static assets)
- **Year 1 Cost**: $0 (free tier)
- **Year 2+ Cost**: ~$4.25/month (50 GB × $0.085)

**Optimization**:
- Cache static assets for 1 year (reduce requests)
- Gzip/Brotli compression (reduce transfer)
- Serve assets from edge locations (faster + cheaper)

**Scaling Threshold**:
- Free tier covers 1 TB/month (sufficient for 2,000 users)
- At 100 GB/month: ~$8.50/month (still under budget)

---

### Route 53 (Optional)

**Pricing**:
- $0.50 per hosted zone/month
- $0.40 per million queries (first billion queries)

**Expected Usage**:
- 1 hosted zone (if using custom domain)
- ~10,000 DNS queries/month
- **Cost**: $0.50/month

**Note**: Only needed if using custom domain (e.g., `streammaxing.com`)

---

### AWS Secrets Manager (Optional)

**Pricing**:
- $0.40 per secret/month
- $0.05 per 10,000 API calls

**Alternative**: Systems Manager Parameter Store (FREE for standard parameters)

**Recommendation**: Use Parameter Store to save $0.40/secret/month

---

### Total Monthly Cost

**Year 1** (with AWS free tier):
| Service           | Cost      |
|-------------------|-----------|
| Neon Database     | $0        |
| Lambda            | $0        |
| API Gateway       | $0        |
| S3                | $0.23     |
| CloudFront        | $0        |
| Route 53          | $0.50*    |
| **Total**         | **$0.73** |

*Optional (custom domain only)

**Year 2+** (without free tier):
| Service           | Cost      |
|-------------------|-----------|
| Neon Database     | $0-$5     |
| Lambda            | $0        |
| API Gateway       | $0.01     |
| S3                | $0.23     |
| CloudFront        | $4.25     |
| Route 53          | $0.50*    |
| **Total**         | **$5-$10**|

*Optional (custom domain only)

**Heavy Usage** (100 streamers, 20 servers, 10,000 notifications/month):
| Service           | Cost      |
|-------------------|-----------|
| Neon Database     | $5        |
| Lambda            | $2        |
| API Gateway       | $0.10     |
| S3                | $0.23     |
| CloudFront        | $8.50     |
| Route 53          | $0.50     |
| **Total**         | **$16.33**|

Still under $20/month target.

---

## Cost Guards & Monitoring

### CloudWatch Billing Alarms

**Setup**:
1. Create SNS topic for billing alerts
2. Create CloudWatch alarms at thresholds:
   - $10/month (warning)
   - $15/month (alert)
   - $20/month (critical)

**AWS CLI**:
```bash
aws cloudwatch put-metric-alarm \
  --alarm-name "Billing-Alert-10-Dollars" \
  --alarm-description "Alert when monthly charges exceed $10" \
  --metric-name EstimatedCharges \
  --namespace AWS/Billing \
  --statistic Maximum \
  --period 21600 \
  --evaluation-periods 1 \
  --threshold 10.0 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=Currency,Value=USD
```

### Lambda Cost Optimization

**Strategies**:
- Use smallest memory size that performs well (256 MB)
- Optimize cold starts (minimize dependencies)
- Avoid provisioned concurrency (on-demand only)
- Set maximum concurrency limit (prevent runaway costs)

**Monitoring**:
- Track average execution time (reduce to < 500ms)
- Monitor cold start frequency (optimize if > 20%)
- Alert on error rate > 5% (errors still consume compute time)

### CloudFront Cost Optimization

**Strategies**:
- Cache static assets for 1 year (`Cache-Control: max-age=31536000`)
- Use Gzip/Brotli compression (reduce transfer by ~70%)
- Set `index.html` to no-cache (always fresh, minimal size)
- Invalidate cache only on deployments (not frequently)

**Monitoring**:
- Track cache hit ratio (target > 80%)
- Monitor data transfer (alert if > 100 GB/month)
- Check CloudFront costs weekly

### Database Cost Optimization

**Strategies**:
- Stay within Neon free tier (5 GB storage, 100 hours compute)
- Use connection pooling (reduce connection overhead)
- Optimize queries (prevent full table scans)
- Delete old `notification_log` entries (> 30 days)

**Monitoring**:
- Track database size (alert at 4 GB - close to free tier limit)
- Monitor compute hours (alert at 80 hours/month)
- Check query performance (alert on slow queries > 100ms)

### API Gateway Cost Optimization

**Strategies**:
- Use HTTP API instead of REST API (3.5x cheaper)
- No caching (CloudFront handles caching)
- Disable access logging (saves CloudWatch costs)

**Monitoring**:
- Track request count (alert at 900K requests/month - close to free tier)

---

## Cost Scaling Scenarios

### Scenario 1: Small Deployment
- 5 Discord servers
- 10 streamers
- 500 notifications/month
- 50 users

**Monthly Cost**: $0.73 (Year 1), $5 (Year 2+)

### Scenario 2: Medium Deployment
- 10 Discord servers
- 30 streamers
- 2,000 notifications/month
- 200 users

**Monthly Cost**: $0.73 (Year 1), $7 (Year 2+)

### Scenario 3: Large Deployment
- 20 Discord servers
- 100 streamers
- 10,000 notifications/month
- 1,000 users

**Monthly Cost**: $1 (Year 1), $16 (Year 2+)

### Scenario 4: Very Large Deployment (Budget Limit)
- 50 Discord servers
- 300 streamers
- 50,000 notifications/month
- 5,000 users

**Monthly Cost**: $3 (Year 1), $25+ (Year 2+) - **Exceeds budget**

**Mitigation**:
- Introduce pricing tier for heavy users
- Optimize CloudFront caching (reduce data transfer)
- Use Neon Pro tier with better pricing (scale pricing)

---

## Unexpected Cost Sources

### 1. VPC Costs (AVOID)
- NAT Gateway: $32/month (DO NOT USE)
- VPC Endpoints: $7/month per endpoint
- **Mitigation**: Use public Lambda (no VPC)

### 2. CloudWatch Logs
- $0.50 per GB ingested
- $0.03 per GB stored
- **Mitigation**: Log errors only (not debug), 1-day retention

### 3. Data Transfer Costs
- S3 to internet (not through CloudFront): $0.09/GB
- **Mitigation**: Use CloudFront for all frontend access

### 4. Lambda Provisioned Concurrency
- $0.015 per GB-hour
- 256 MB provisioned 24/7 = $2.88/month
- **Mitigation**: Use on-demand only (no provisioning)

### 5. Unused Resources
- Old Lambda versions (storage costs)
- Old S3 versions (if versioning enabled)
- **Mitigation**: Delete old versions, lifecycle policies

---

## Cost Monitoring Dashboard

### Key Metrics to Track

**Daily**:
- AWS Billing (total charges to date)
- Lambda invocations
- CloudFront data transfer

**Weekly**:
- Neon database size
- Neon compute hours used
- API Gateway request count
- CloudWatch log volume

**Monthly**:
- Total AWS bill (vs $20 budget)
- Cost per service breakdown
- Cost per user (total cost / active users)

### Alerts

**Critical**:
- Monthly bill > $20
- Lambda invocations > 900K (approaching free tier limit)
- Database size > 4.5 GB (approaching free tier limit)

**Warning**:
- Monthly bill > $15
- CloudFront data transfer > 80 GB/month
- Neon compute hours > 80/month

**Info**:
- Monthly bill > $10
- Unusual spike in Lambda invocations

---

## Cost Reduction Strategies

### Immediate Actions
1. Use Parameter Store instead of Secrets Manager (save $0.40/secret/month)
2. Disable API Gateway access logging (save ~$0.50/month)
3. Set CloudWatch log retention to 1 day (save ~$1/month)
4. Use HTTP API instead of REST API (save 71% on API Gateway costs)

### Long-Term Optimizations
1. Delete `notification_log` entries > 30 days (reduce DB size)
2. Implement user-based rate limiting (prevent abuse)
3. Add caching layer (Redis on Upstash free tier) for Discord API responses
4. Optimize Lambda memory size (benchmark 128 MB vs 256 MB)
5. Implement pagination for large queries (reduce DB compute)

### Future Revenue Options
- Freemium model: Free tier (5 streamers) + Paid tier ($5/month for unlimited)
- One-time setup fee for custom message templates
- Donate button for users who want to support the project

---

## Emergency Cost Controls

### If Monthly Bill Exceeds $20

**Immediate Actions**:
1. Check AWS Cost Explorer for cost spike source
2. Disable non-essential features (e.g., detailed logging)
3. Set Lambda reserved concurrency to 10 (prevent runaway invocations)
4. Pause EventSub subscriptions temporarily (if spike from webhooks)

**Investigation**:
1. Check for DDoS/abuse (unusual webhook traffic)
2. Review CloudWatch logs for errors causing retries
3. Check for forgotten resources (old Lambda versions, test instances)

**Long-Term Fix**:
1. Optimize the service causing cost spike
2. Introduce usage limits (max 100 streamers per deployment)
3. Consider alternative architecture (serverless → container on Fly.io)

---

## Cost Comparison: Alternatives

### Hosting Alternatives

**Fly.io** (Container Hosting):
- $1.94/month (shared-cpu-1x, 256 MB RAM)
- $0.15/GB egress (vs CloudFront $0.085/GB)
- Total: ~$10/month (similar to AWS Year 2+)

**Railway** (Container Hosting):
- $5/month credit (free tier)
- $0.000463/GB-second (compute)
- $0.10/GB egress
- Total: $0-$10/month

**Vercel** (Serverless):
- Free tier: 100 GB bandwidth, unlimited functions
- Pro tier: $20/month (1 TB bandwidth)
- Total: $0 (free tier), $20 (pro tier)

**Recommendation**: AWS is most cost-effective for Year 1 (free tier), competitive for Year 2+ with optimization.

---

## Summary

**Current Architecture**:
- Year 1: $0.73/month (with free tier)
- Year 2+: $5-$10/month (typical usage)
- Heavy usage: $15-$20/month (within budget)

**Key Cost Drivers**:
1. CloudFront data transfer (Year 2+)
2. Neon database (if exceeding free tier)
3. Lambda invocations (negligible within free tier)

**Cost Guards**:
- CloudWatch billing alarms ($10, $15, $20)
- Usage limits (max streamers, rate limiting)
- Monitoring dashboard (daily/weekly checks)

**Scaling Path**:
- Stay under $20/month up to 50 servers, 100 streamers
- Beyond that, consider monetization or cost optimization (caching, CDN alternatives)
