# System Architecture

## Overview

StreamMaxing v3 is a serverless notification system that bridges Twitch and Discord. When a Twitch streamer goes live, the system sends customizable notifications to configured Discord channels across multiple servers.

## High-Level Architecture

```
┌─────────────────┐
│  Discord Users  │
└────────┬────────┘
         │ (1) Login with Discord
         ▼
┌─────────────────────────────────────────┐
│  Frontend (React + TypeScript)          │
│  Hosted on S3 + CloudFront              │
│  - Admin Dashboard                      │
│  - User Settings                        │
│  - Template Editor                      │
└────────┬────────────────────────────────┘
         │ (2) API Calls (HTTPS)
         ▼
┌─────────────────────────────────────────┐
│  API Gateway (HTTP API)                 │
│  Routes: /api/*, /webhooks/*            │
└────────┬────────────────────────────────┘
         │ (3) Invoke Lambda
         ▼
┌─────────────────────────────────────────┐
│  AWS Lambda (Go 1.22+)                  │
│  - HTTP Handlers                        │
│  - Business Logic                       │
│  - OAuth Flows                          │
│  - Webhook Processing                   │
└────┬────────────────────────────────┬───┘
     │                                │
     │ (4) Database Queries           │ (5) External API Calls
     ▼                                ▼
┌──────────────────┐    ┌──────────────────────────┐
│  Neon Postgres   │    │  External Services       │
│  (Serverless)    │    │  - Discord API           │
│  - Users         │    │  - Twitch API            │
│  - Guilds        │    │  - Twitch EventSub       │
│  - Streamers     │    └──────────────────────────┘
│  - Preferences   │
└──────────────────┘

         ▲ (6) Webhook Events
         │
┌────────┴────────┐
│  Twitch         │
│  EventSub       │
│  (stream.online)│
└─────────────────┘
```

## Component Details

### Frontend (S3 + CloudFront)

**Technology**: React 18 + TypeScript + Vite

**Hosting**:
- **S3 Bucket**: Stores built static files (HTML, JS, CSS, assets)
- **CloudFront**: CDN distribution in front of S3
  - HTTPS by default
  - Global edge caching
  - Custom domain support (optional)
  - Origin Access Control (OAC) for secure S3 access
  - Cache invalidation on deployments

**Caching Strategy**:
- `/assets/*` → Cache for 1 year (immutable, hashed filenames)
- `/index.html` → No cache (always fetch latest)
- `/api/*` → Forward to API Gateway (no caching)

**Pages**:
1. Landing page with Discord login
2. Admin dashboard (server selection, channel config, streamer management, template editor)
3. User settings (notification preferences)

### API Gateway (HTTP API)

**Type**: HTTP API (cheaper than REST API)

**Configuration**:
- Route: `ANY /{proxy+}` → Lambda integration
- CORS enabled for CloudFront domain
- Custom domain optional (requires Route 53 + ACM certificate)

**Endpoints**:
- `/api/auth/*` - OAuth flows
- `/api/guilds/*` - Guild management
- `/api/users/*` - User preferences
- `/api/streamers/*` - Streamer management
- `/webhooks/twitch` - EventSub webhook
- `/webhooks/discord` - Discord events (optional)
- `/api/health` - Health check

### Lambda Function (Go)

**Runtime**: Go 1.22+ (compiled to `provided.al2`)

**Configuration**:
- Memory: 256 MB
- Timeout: 30 seconds
- Environment: Production (or dev/staging)
- Concurrency: 100 (default)

**Modules**:
1. **Config**: Centralized configuration loader (`internal/config/`) — reads all env vars and secrets in one place, then injects values into services via dependency injection. No `os.Getenv` calls in services/handlers/middleware.
2. **Handlers**: HTTP request handlers for each route
3. **Services**: Business logic (Discord, Twitch, Notifications)
4. **Database**: Query functions and models
5. **Middleware**: Auth validation, CORS, rate limiting, error handling

**Execution Flow**:
1. API Gateway invokes Lambda with HTTP event
2. Router matches path to handler
3. Middleware runs (auth, CORS)
4. Handler processes request
5. Service layer executes business logic
6. Database layer queries Neon Postgres
7. Response returned to API Gateway → Client

### Database (Neon Postgres)

**Type**: Serverless Postgres

**Connection**:
- Pooled connections optimized for Lambda
- Uses pgx driver (Go)
- Database URL loaded from centralized config (`internal/config/`)

**Tables** (see [database.md](database.md) for full schema):
- `users` - Discord users
- `guilds` - Discord servers
- `guild_config` - Per-server notification settings
- `streamers` - Twitch streamers
- `guild_streamers` - Junction table (which streamers per guild)
- `user_preferences` - Per-user notification toggles
- `eventsub_subscriptions` - Twitch EventSub subscriptions
- `notification_log` - Idempotency tracking

### External Integrations

**Discord API**:
- OAuth 2.0 for user authentication
- Fetch user's guilds, channels, roles
- Send messages via bot token
- Handle webhook events (GUILD_DELETE, GUILD_MEMBER_REMOVE) - optional

**Twitch API**:
- OAuth 2.0 for streamer authentication
- Create EventSub subscriptions
- Receive webhook events (stream.online)
- Verify webhook signatures (HMAC-SHA256)

## Event Flows

### Flow 1: User Onboarding

```
User → Frontend: Click "Login with Discord"
Frontend → Discord OAuth: Redirect to Discord authorization
Discord → Frontend: Callback with code
Frontend → Backend: /api/auth/discord/callback?code=...
Backend → Discord API: Exchange code for token
Backend → Discord API: Fetch user info and guilds
Backend → Database: INSERT user, guilds
Backend → Frontend: Set session cookie, redirect to dashboard
```

### Flow 2: Bot Installation

```
User → Frontend: Select server, click "Install Bot"
Frontend → Discord OAuth: Redirect with bot scope + guild_id
Discord → Frontend: Bot installed callback
Frontend → Backend: /api/guilds/:id/config (fetch default config)
Backend → Database: INSERT guild_config with defaults
```

### Flow 3: Streamer Linking

```
Admin → Frontend: Click "Add Streamer"
Frontend → Twitch OAuth: Redirect to Twitch authorization
Twitch → Frontend: Callback with code
Frontend → Backend: /api/auth/twitch/callback?code=...
Backend → Twitch API: Exchange code for token
Backend → Twitch API: Get broadcaster_user_id
Backend → Database: INSERT streamer
Backend → Twitch API: Create EventSub subscription (stream.online)
Backend → Database: INSERT eventsub_subscription
Backend → Frontend: Success, show streamer in list
```

### Flow 4: Notification Delivery

```
Twitch EventSub → Backend: POST /webhooks/twitch (stream.online event)
Backend: Verify HMAC signature
Backend → Database: Query guild_streamers for guilds tracking this streamer
Backend → Database: Query guild_config for each guild
Backend → Database: Query user_preferences for opted-out users
Backend → Database: Check notification_log for duplicate (idempotency)
Backend: Render message template with streamer data
Backend → Discord API: Send message to channel (exclude opted-out users)
Backend → Database: INSERT notification_log
Backend → Twitch: Return 200 OK
```

### Flow 5: User Preferences Update

```
User → Frontend: Toggle notification for streamer X in server Y
Frontend → Backend: PUT /api/users/me/preferences/:guild_id/:streamer_id
Backend: Validate user is member of guild
Backend → Database: UPDATE user_preferences SET notifications_enabled = false
Backend → Frontend: Success
```

## Security

### Authentication
- **Discord OAuth**: `identify` and `guilds` scopes
- **Twitch OAuth**: `user:read:email` scope
- **Session Management**: JWT tokens stored in HTTP-only cookies with 24-hour expiration
- **API Authorization**: Middleware validates JWT on protected routes
- **Session Revocation**: Revoked sessions tracked in database for immediate invalidation
- **JWT Security**: Unique JWT ID (jti) per session, SameSite=Strict cookies

### Authorization
- **Real-Time Validation**: Guild permissions re-validated on every request via Discord API
- **Permission Caching**: Short-lived cache (5 minutes) to balance security and performance
- **Principle of Least Privilege**: Separate middleware for admin and member access
- **Audit Logging**: All permission denials logged for security monitoring

### Webhook Security
- **Twitch EventSub**: HMAC-SHA256 signature verification with constant-time comparison
- **Replay Attack Prevention**: Timestamp validation (10-minute window)
- **Idempotency**: Message ID tracking prevents duplicate processing
- **Rate Limiting**: Webhook-specific rate limiting (100 requests/second)
- **Secret Storage**: AWS Secrets Manager with automatic rotation

### Data Protection
- **OAuth Token Encryption**: All Twitch tokens encrypted at rest using AWS KMS (AES-256)
- **Secrets Management**: All secrets stored in AWS Secrets Manager (JWT secret, OAuth credentials)
- **SQL Injection**: Parameterized queries with pgx (all queries audited)
- **XSS Protection**: React escapes user input by default, template validation on backend
- **Input Validation**: Comprehensive validation for all user inputs (format, length, content)
- **CORS**: Restricted to CloudFront domain, no wildcard origins in production

### Rate Limiting
- **Per-User Limits**: 50 requests/minute per authenticated user
- **Global Limits**: 1000 requests/second across all users
- **Webhook Limits**: 100 webhook events/second
- **Response Headers**: `Retry-After` headers on rate limit violations

### Security Monitoring
- **Structured Logging**: JSON-formatted security events with severity levels
- **CloudWatch Metrics**: Real-time metrics for auth failures, permission denials, rate limits
- **Automated Alerts**: CloudWatch alarms for security anomalies
- **Audit Trail**: Comprehensive audit log for all sensitive operations

### Security Events Tracked
- Authentication attempts (success/failure)
- Authorization denials
- Rate limit violations
- Webhook signature failures
- Token encryption/decryption errors
- Anomalous activity patterns
- Admin actions (config changes, streamer management)

## Scalability

### Current Limits
- Lambda concurrency: 100 (default, can increase)
- Neon connections: 100 (free tier)
- API Gateway: 10,000 requests/second (default)
- EventSub: 10,000 subscriptions per client (plenty for this use case)

### Bottlenecks
- Database connection pool (mitigated by serverless driver)
- Discord API rate limits (10,000 requests per 10 minutes per bot)
- Lambda cold starts (~1-2 seconds for Go)

### Cost Scaling
- Linear with number of notifications sent
- Sublinear with traffic (Lambda free tier, CloudFront caching)
- Database storage grows with users/guilds (minimal cost)

## Deployment Architecture

```
┌─────────────────────────────────────────┐
│  Developer Workstation                  │
│  - Backend: Go code                     │
│  - Frontend: React code                 │
└────────┬────────────────────────────────┘
         │
         │ (1) Build
         ▼
┌─────────────────────────────────────────┐
│  Build Artifacts                        │
│  - Lambda: bootstrap binary (zip)       │
│  - Frontend: dist/ folder (HTML/JS/CSS) │
└────────┬────────────────────────────────┘
         │
         │ (2) Deploy
         ▼
┌─────────────────────────────────────────┐
│  AWS Infrastructure                     │
│  - Lambda: Upload zip                   │
│  - S3: Sync dist/ folder                │
│  - CloudFront: Invalidate cache         │
└─────────────────────────────────────────┘
```

**Deployment Steps**:
1. Backend: `GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/lambda/main.go`
2. Backend: `zip deployment.zip bootstrap`
3. Backend: `aws lambda update-function-code --function-name streammaxing --zip-file fileb://deployment.zip`
4. Frontend: `npm run build`
5. Frontend: `aws s3 sync dist/ s3://streammaxing-frontend --delete`
6. Frontend: `aws cloudfront create-invalidation --distribution-id XXX --paths "/*"`

## Monitoring

### Metrics to Track
- Lambda invocations (success/error rate)
- API Gateway 4xx/5xx errors
- Database connection pool usage
- EventSub subscription status
- Discord API rate limit hits
- CloudFront cache hit ratio
- **Security Metrics** (added in Task 007):
  - Authentication failures (alert threshold: >50 per 5 minutes)
  - Permission denials (alert threshold: >20 per 5 minutes)
  - Rate limit violations (alert threshold: >100 per 5 minutes)
  - Webhook signature failures (alert threshold: >5 per minute)
  - Token encryption/decryption errors (alert immediately)

### Logging
- CloudWatch Logs for Lambda (structured JSON format)
- Security event logs with severity levels (INFO, WARNING, CRITICAL)
- API Gateway access logs (optional)
- Audit logs for sensitive operations (stored in database)

### Alerting
- CloudWatch billing alarms ($10, $15, $20 thresholds)
- Lambda error rate > 5%
- Database connection errors
- EventSub subscription failures
- **Security Alarms** (added in Task 007):
  - High authentication failure rate (potential brute force)
  - Webhook signature verification failures (potential attack)
  - Rate limit abuse (potential DDoS)
  - Anomalous permission denial patterns

## Disaster Recovery

### Backup Strategy
- Database: Neon automatic backups (point-in-time recovery)
- Code: Git repository (GitHub)
- Configuration: Infrastructure as Code (SAM/Terraform)

### Failure Scenarios
1. **Lambda failure**: Auto-retry by AWS, manual redeploy if persistent
2. **Database outage**: Neon SLA 99.9%, wait for recovery
3. **Discord API down**: Queue notifications (future: SQS)
4. **EventSub webhook failures**: Twitch retries automatically

## Future Enhancements

1. **Multi-region deployment**: CloudFront already global, consider Lambda@Edge
2. **Notification queue**: SQS for buffering high-volume events
3. **Analytics dashboard**: Track notification delivery rates, user engagement
4. **Horizontal scaling**: DynamoDB for session storage (if JWT doesn't scale)
5. **CDN optimization**: Preload critical assets, optimize bundle size
