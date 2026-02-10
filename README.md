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

## Project Structure
```
.agent/              # Documentation
├── README.md        # Documentation index
├── System/          # System architecture docs
├── Tasks/           # Implementation task specs
└── SOP/             # Standard operating procedures

backend/             # Go backend
├── cmd/lambda/      # Lambda entry point
├── internal/        # Application code
└── migrations/      # Database migrations

frontend/            # React frontend
├── src/             # Source code
└── public/          # Static assets
```

## Getting Started
1. Read `.agent/README.md` for documentation overview
2. Follow `.agent/SOP/local-development.md` for local setup
3. Review `.agent/System/architecture.md` for system design

## License
MIT
