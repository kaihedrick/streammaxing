# Technology Stack

## Backend

### Language & Runtime
- **Go**: 1.22 or higher
  - Chosen for fast cold starts, small binary size, excellent concurrency
  - Native AWS Lambda support
  - Strong typing and performance

### Framework
- **net/http**: Standard library HTTP server (no framework)
  - Minimal dependencies for faster cold starts
  - Sufficient for simple routing needs
  - Lower maintenance overhead

### Database
- **Neon Postgres**: Serverless Postgres
  - **Driver**: `github.com/jackc/pgx/v5`
  - Optimized for serverless (connection pooling via HTTP)
  - Free tier: 5 GB storage, 0.5 compute units
  - Starter tier: $5/month for more resources

### Key Dependencies
```go
require (
    github.com/jackc/pgx/v5 v5.5.0           // Postgres driver
    github.com/golang-jwt/jwt/v5 v5.0.0      // JWT tokens
    github.com/aws/aws-lambda-go v1.41.0     // Lambda runtime
)
```

### OAuth Libraries
- Discord OAuth: Manual implementation using `net/http` and `encoding/json`
- Twitch OAuth: Manual implementation (lightweight, no SDK needed)

---

## Frontend

### Core Framework
- **React**: 18.x
- **TypeScript**: 5.x
- **Vite**: 5.x (build tool and dev server)

### Routing
- **React Router**: 6.x
  - Client-side routing
  - Protected routes for authenticated users

### HTTP Client
- **Axios**: 1.x or native `fetch` API
  - API communication with backend
  - Interceptors for auth tokens

### UI Styling
- **Option 1**: Vanilla CSS (minimal, fast)
- **Option 2**: Tailwind CSS (utility-first, easy to customize)
- **Option 3**: MUI (Material-UI) (component library)

**Recommended**: Vanilla CSS or Tailwind for minimal bundle size

### Build Configuration
```json
{
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.20.0",
    "axios": "^1.6.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "typescript": "^5.3.0",
    "vite": "^5.0.0"
  }
}
```

---

## Infrastructure

### Hosting

#### Backend
- **AWS Lambda**
  - Runtime: `provided.al2` (custom Go runtime)
  - Memory: 256 MB
  - Timeout: 30 seconds
  - Provisioned concurrency: 0 (cost savings)

#### API Gateway
- **HTTP API** (not REST API)
  - Lower cost ($1/million requests vs $3.50)
  - Sufficient features for this use case
  - Native CORS support
  - Custom domain optional (Route 53 + ACM)

#### Frontend
- **S3 Bucket**
  - Static website hosting (private, accessed via CloudFront)
  - Versioning enabled for rollback capability
  - Lifecycle rules for old versions (delete after 30 days)

- **CloudFront Distribution**
  - Global CDN for fast loading
  - HTTPS by default (free certificate via ACM)
  - Origin Access Control (OAC) for secure S3 access
  - Custom domain support (optional)
  - Cache behaviors:
    - `/assets/*` → Cache 1 year (immutable)
    - `/index.html` → No cache
    - `/api/*` → Forward to API Gateway (no caching)

#### Database
- **Neon Postgres**
  - Serverless (auto-scaling compute)
  - Branching support (dev/staging/prod databases)
  - Point-in-time recovery
  - Free tier sufficient for initial deployment

### Secrets Management
- **Option 1**: AWS Secrets Manager ($0.40/secret/month)
- **Option 2**: AWS Systems Manager Parameter Store (free for standard parameters)
- **Recommended**: Parameter Store for cost savings

### Monitoring
- **CloudWatch Logs**: Lambda logs (error level only)
- **CloudWatch Metrics**: Lambda invocations, API Gateway requests
- **CloudWatch Alarms**: Billing alerts, error rate alerts

---

## External Services

### Discord
- **Discord Developer Portal**: Create bot application
  - Bot token
  - OAuth2 client ID/secret
  - Required scopes: `identify`, `guilds`, `bot`
  - Required bot permissions: `Send Messages`, `Embed Links`, `Mention Everyone`

### Twitch
- **Twitch Developer Console**: Create application
  - Client ID
  - Client secret
  - OAuth redirect URI: `https://your-api-url/api/auth/twitch/callback`
  - EventSub webhook URL: `https://your-api-url/webhooks/twitch`

---

## Development Tools

### Required
- **Go**: 1.22+ (`go version`)
- **Node.js**: 18+ LTS (`node -v`)
- **npm**: 9+ (`npm -v`)
- **Git**: 2.x (`git --version`)
- **AWS CLI**: 2.x (`aws --version`)

### Optional
- **Docker**: For local Postgres testing (alternative to Neon branch)
- **Postman/Insomnia**: API testing
- **VS Code**: Recommended editor with Go and React extensions

### VS Code Extensions (Recommended)
- **Go** (`golang.go`)
- **ESLint** (`dbaeumer.vscode-eslint`)
- **Prettier** (`esbenp.prettier-vscode`)
- **TypeScript Vue Plugin** (if using Vue) or React snippets
- **AWS Toolkit** (`amazonwebservices.aws-toolkit-vscode`)

---

## Version Control

### Git
- **Repository**: GitHub (or GitLab/Bitbucket)
- **Branching Strategy**: Simple workflow
  - `main` branch for production
  - Feature branches for development
  - Squash merge to keep history clean

### .gitignore
```
# Backend
backend/bootstrap
backend/deployment.zip
*.exe
*.exe~
*.dll
*.so
*.dylib

# Frontend
frontend/node_modules/
frontend/dist/
frontend/.env.local

# Environment
.env
.env.local
.env.production

# IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

---

## Environment Variables

### Backend (.env)
```bash
# Database
DATABASE_URL=postgresql://user:pass@neon.tech:5432/dbname

# Discord
DISCORD_CLIENT_ID=123456789
DISCORD_CLIENT_SECRET=abc123
DISCORD_BOT_TOKEN=MTk...

# Twitch
TWITCH_CLIENT_ID=abc123def456
TWITCH_CLIENT_SECRET=secret123
TWITCH_WEBHOOK_SECRET=random_secret_for_signatures

# App Config
API_BASE_URL=https://api-id.execute-api.us-east-1.amazonaws.com
FRONTEND_URL=https://d1234567890.cloudfront.net
JWT_SECRET=random_jwt_secret_min_32_chars

# Environment
ENVIRONMENT=production
LOG_LEVEL=info
```

### Frontend (.env)
```bash
VITE_API_URL=https://api-id.execute-api.us-east-1.amazonaws.com
```

---

## Build Process

### Backend
```bash
# Install dependencies
go mod download

# Build for Lambda
GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/lambda/main.go

# Create deployment package
zip deployment.zip bootstrap

# Deploy to Lambda
aws lambda update-function-code \
  --function-name streammaxing \
  --zip-file fileb://deployment.zip
```

### Frontend
```bash
# Install dependencies
npm install

# Development server
npm run dev

# Production build
npm run build

# Preview production build
npm run preview

# Deploy to S3
aws s3 sync dist/ s3://streammaxing-frontend --delete

# Invalidate CloudFront cache
aws cloudfront create-invalidation \
  --distribution-id E123456789ABCD \
  --paths "/*"
```

---

## Testing

### Backend Testing
- **Unit Tests**: `go test ./...`
- **Coverage**: `go test -cover ./...`
- **Test Files**: `*_test.go` in same package

### Frontend Testing
- **Option 1**: Vitest (Vite-native test runner)
- **Option 2**: Jest + React Testing Library
- **Recommended**: Vitest for consistency with Vite

### Integration Testing
- **Manual Testing**: Use Postman/Insomnia for API testing
- **End-to-End**: Simulate OAuth flows with test accounts
- **Webhook Testing**: Use Twitch CLI for local EventSub testing

---

## Performance Requirements

### Backend
- **Cold Start**: < 2 seconds (Go typically ~1 second)
- **Warm Response**: < 200ms (database queries optimized)
- **Webhook Processing**: < 500ms (return 200 to Twitch immediately)

### Frontend
- **First Contentful Paint**: < 1.5 seconds
- **Time to Interactive**: < 3 seconds
- **Bundle Size**: < 500 KB (gzipped)

### Database
- **Query Time**: < 50ms for simple queries
- **Connection Pool**: 10-20 connections (Lambda concurrency)

---

## Security Requirements

### Backend
- HTTPS only (enforced by API Gateway)
- Validate all user input
- Parameterized SQL queries (prevent SQL injection)
- HMAC signature verification for webhooks
- JWT token expiration (7 days)
- HTTP-only cookies for session storage

### Frontend
- CSP (Content Security Policy) headers
- React escapes user input (XSS prevention)
- HTTPS only (enforced by CloudFront)
- No sensitive data in localStorage (use HTTP-only cookies)

### Infrastructure
- S3 bucket private (accessed via CloudFront OAC)
- Lambda environment variables encrypted at rest
- Secrets in Parameter Store (encrypted)
- IAM roles with least privilege

---

## Cost Optimization

### Backend
- Use HTTP API instead of REST API (3.5x cheaper)
- Minimize Lambda memory (256 MB sufficient)
- No provisioned concurrency (on-demand only)
- Error-level logging only (reduce CloudWatch costs)

### Frontend
- CloudFront caching reduces S3/Lambda requests
- Gzip/Brotli compression reduces bandwidth
- Tree shaking removes unused code

### Database
- Stay within Neon free tier if possible
- Index critical queries for performance
- Avoid full table scans

---

## Compliance & Licensing

### Open Source Dependencies
- All dependencies use permissive licenses (MIT, Apache 2.0, BSD)
- No GPL dependencies (avoid copyleft)

### Data Privacy
- GDPR compliance: Users can delete their data
- Discord/Twitch OAuth: Follow platform TOS
- No user tracking or analytics (unless explicitly added)

---

## Version History

- **v3.0.0**: Complete rewrite with multi-server support, user preferences, advanced templates
- **v2.0.0**: (Legacy) Single-server implementation
- **v1.0.0**: (Legacy) Proof of concept
