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

## Project Structure
```
backend/
├── cmd/lambda/main.go       # Lambda entry point
├── internal/
│   ├── handlers/            # HTTP request handlers
│   ├── services/            # Business logic
│   ├── db/                  # Database layer
│   └── middleware/          # HTTP middleware
├── migrations/              # SQL migrations
└── go.mod                   # Go module definition
```

## Environment Variables
See `.env.example` for required configuration.
