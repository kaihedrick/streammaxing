# Task 001: Project Bootstrap

## Status
Complete

## Overview
Initialize the project structure, package managers, and core configuration files for both backend (Go) and frontend (React + TypeScript).

---

## Goals
1. Create folder structure for backend and frontend
2. Initialize Go module for backend
3. Initialize React + Vite for frontend
4. Set up configuration files (.gitignore, .env.example, README.md)
5. Verify build environment

---

## Backend Setup

### Folder Structure
```
backend/
├── cmd/
│   └── lambda/
│       └── main.go          # Lambda entry point
├── internal/
│   ├── handlers/            # HTTP request handlers
│   ├── services/            # Business logic
│   │   ├── discord/
│   │   ├── twitch/
│   │   └── notifications/
│   ├── db/                  # Database layer
│   └── middleware/          # HTTP middleware
├── migrations/              # SQL migration files
├── go.mod
├── go.sum
├── .env.example
└── README.md
```

### Initialize Go Module
```bash
cd backend
go mod init github.com/yourusername/streammaxing
```

### Initial Dependencies
```bash
go get github.com/jackc/pgx/v5
go get github.com/golang-jwt/jwt/v5
go get github.com/aws/aws-lambda-go
```

### .env.example
```bash
# Database
DATABASE_URL=postgresql://user:pass@neon.tech:5432/dbname

# Discord
DISCORD_CLIENT_ID=
DISCORD_CLIENT_SECRET=
DISCORD_BOT_TOKEN=

# Twitch
TWITCH_CLIENT_ID=
TWITCH_CLIENT_SECRET=
TWITCH_WEBHOOK_SECRET=

# App Config
API_BASE_URL=https://your-api-gateway-url.execute-api.us-east-1.amazonaws.com
FRONTEND_URL=https://your-cloudfront-url.cloudfront.net
JWT_SECRET=

# Environment
ENVIRONMENT=development
LOG_LEVEL=info
```

### Backend README.md
```markdown
# StreamMaxing Backend

Go-based serverless backend for Twitch → Discord notifications.

## Setup
1. Copy `.env.example` to `.env` and fill in credentials
2. Install dependencies: `go mod download`
3. Run locally: `go run cmd/lambda/main.go`
4. Build for Lambda: `GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/lambda/main.go`

## Deployment
```bash
zip deployment.zip bootstrap
aws lambda update-function-code --function-name streammaxing --zip-file fileb://deployment.zip
```

---

## Frontend Setup

### Create React App with Vite
```bash
npm create vite@latest frontend -- --template react-ts
cd frontend
npm install
```

### Folder Structure
```
frontend/
├── src/
│   ├── components/
│   │   ├── Auth/
│   │   ├── Dashboard/
│   │   ├── Settings/
│   │   └── common/
│   ├── services/
│   │   └── api.ts
│   ├── types/
│   │   └── index.ts
│   ├── App.tsx
│   ├── main.tsx
│   └── vite-env.d.ts
├── public/
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
├── .env.example
└── README.md
```

### Additional Dependencies
```bash
npm install react-router-dom axios
npm install --save-dev @types/react-router-dom
```

### .env.example
```bash
VITE_API_URL=https://your-api-gateway-url.execute-api.us-east-1.amazonaws.com
```

### Frontend README.md
```markdown
# StreamMaxing Frontend

React + TypeScript frontend for managing Twitch notification settings.

## Setup
1. Copy `.env.example` to `.env` and set `VITE_API_URL`
2. Install dependencies: `npm install`
3. Run dev server: `npm run dev`
4. Build for production: `npm run build`

## Deployment
```bash
npm run build
aws s3 sync dist/ s3://streammaxing-frontend --delete
aws cloudfront create-invalidation --distribution-id XXX --paths "/*"
```

---

## Root-Level Files

### .gitignore
```
# Backend
backend/bootstrap
backend/deployment.zip
backend/*.exe
backend/*.dll
backend/*.so
backend/*.dylib

# Frontend
frontend/node_modules/
frontend/dist/
frontend/.env.local
frontend/.env.production

# Environment
.env
.env.local
.env.production

# IDE
.vscode/
.idea/
*.swp
*.swo
*.iml

# OS
.DS_Store
Thumbs.db

# Logs
*.log

# Agent/Claude
.claude/
```

### Root README.md
```markdown
# StreamMaxing v3

Serverless Twitch → Discord notification system with multi-server support.

## Quick Start

### Prerequisites
- Go 1.22+
- Node.js 18+
- AWS CLI configured
- Neon Postgres account
- Discord bot application
- Twitch application

### Setup

**Backend**:
```bash
cd backend
cp .env.example .env
# Fill in .env with credentials
go mod download
go run cmd/lambda/main.go
```

**Frontend**:
```bash
cd frontend
cp .env.example .env
# Set VITE_API_URL
npm install
npm run dev
```

### Documentation
See `.agent/README.md` for full documentation index.

## Architecture
- **Backend**: Go 1.22+ on AWS Lambda
- **Frontend**: React + TypeScript on S3 + CloudFront
- **Database**: Neon Postgres
- **Cost**: <$20/month

## Features
- Multi-server Discord support
- User-level notification preferences
- Advanced message templates with embeds
- Real-time EventSub notifications
```

---

## Verification Checklist

- [x] Backend folder structure created
- [x] `go.mod` initialized
- [x] Backend dependencies installed
- [x] Frontend created with Vite
- [x] Frontend dependencies installed
- [x] .gitignore covers all sensitive files
- [x] .env.example templates created
- [x] README files created
- [x] Backend compiles: `go build ./cmd/lambda`
- [x] Frontend builds: `npm run build`

---

## Next Steps

After bootstrap:
1. Create database schema (Task 002)
2. Implement backend core infrastructure (Task 003)
3. Implement Discord OAuth (Task 004)

---

## Notes

- Do NOT commit .env files
- Use `backend/.env` for backend config
- Use `frontend/.env` for frontend config
- Root-level `.env` is for shared config (currently unused)
