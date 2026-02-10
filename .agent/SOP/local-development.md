# SOP: Local Development

## Overview
This document describes how to set up and run StreamMaxing locally for development.

---

## Prerequisites

### Required Software
- **Go 1.22+**: `go version`
- **Node.js 18+**: `node --version`
- **PostgreSQL Client** (psql): For database access
- **Git**: For version control
- **ngrok** (optional): For testing webhooks locally

### Recommended Tools
- **VS Code** with Go extension
- **Postman** or **curl**: For API testing
- **Twitch CLI**: For testing EventSub webhooks

---

## Initial Setup

### 1. Clone Repository

```bash
git clone https://github.com/yourusername/streammaxing.git
cd streammaxing
```

---

### 2. Set Up Backend

#### Install Go Dependencies

```bash
cd backend
go mod download
```

#### Create Environment File

```bash
cp .env.example .env
```

#### Configure Environment Variables

**File**: `backend/.env`

```bash
# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/streammaxing_dev

# Discord
DISCORD_CLIENT_ID=your_client_id
DISCORD_CLIENT_SECRET=your_client_secret
DISCORD_BOT_TOKEN=your_bot_token

# Twitch
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret
TWITCH_WEBHOOK_SECRET=your_random_secret

# App Config
API_BASE_URL=http://localhost:3000
FRONTEND_URL=http://localhost:5173
JWT_SECRET=your_random_secret_min_32_chars

# Environment
ENVIRONMENT=development
LOG_LEVEL=debug
```

---

### 3. Set Up Database

#### Option A: Local PostgreSQL

**Install PostgreSQL**:

```bash
# macOS
brew install postgresql@15
brew services start postgresql@15

# Ubuntu
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql

# Windows
# Download installer from postgresql.org
```

**Create Database**:

```bash
psql postgres
CREATE DATABASE streammaxing_dev;
\q
```

**Update DATABASE_URL**:
```bash
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/streammaxing_dev
```

---

#### Option B: Neon (Cloud)

1. Sign up at https://neon.tech
2. Create a new project
3. Copy connection string
4. Update `DATABASE_URL` in `.env`

---

#### Apply Migrations

```bash
# Apply all migrations
for migration in migrations/*.sql; do
    psql $DATABASE_URL -f $migration
done

# Or manually
psql $DATABASE_URL -f migrations/001_initial_schema.sql
```

**Verify Tables Created**:

```bash
psql $DATABASE_URL -c "\dt"
```

Expected output:
```
               List of relations
 Schema |         Name          | Type  |  Owner
--------+-----------------------+-------+----------
 public | users                 | table | postgres
 public | guilds                | table | postgres
 public | guild_config          | table | postgres
 public | streamers             | table | postgres
 public | guild_streamers       | table | postgres
 public | user_preferences      | table | postgres
 public | eventsub_subscriptions| table | postgres
 public | notification_log      | table | postgres
```

---

### 4. Set Up Frontend

#### Install Dependencies

```bash
cd frontend
npm install
```

#### Create Environment File

```bash
cp .env.example .env
```

#### Configure Environment Variables

**File**: `frontend/.env`

```bash
VITE_API_URL=http://localhost:3000
```

---

### 5. Set Up Discord Application

1. Go to https://discord.com/developers/applications
2. Create new application
3. Navigate to **OAuth2** → **General**
4. Add redirect URI: `http://localhost:3000/api/auth/discord/callback`
5. Navigate to **Bot**
6. Enable bot and copy token
7. Enable "Server Members Intent"
8. Copy Client ID and Client Secret to `.env`

---

### 6. Set Up Twitch Application

1. Go to https://dev.twitch.tv/console/apps
2. Create new application
3. Add OAuth redirect URI: `http://localhost:3000/api/auth/twitch/callback`
4. Copy Client ID and Client Secret to `.env`
5. Generate a random webhook secret (32+ characters)

---

## Running Locally

### 1. Start Backend

```bash
cd backend
go run cmd/lambda/main.go
```

Expected output:
```
2024/01/15 10:00:00 Starting server on :3000
2024/01/15 10:00:00 Connected to database
```

**Test Backend**:
```bash
curl http://localhost:3000/api/health
```

Expected response:
```json
{"status":"ok"}
```

---

### 2. Start Frontend

In a new terminal:

```bash
cd frontend
npm run dev
```

Expected output:
```
  VITE v5.0.0  ready in 500 ms

  ➜  Local:   http://localhost:5173/
  ➜  Network: use --host to expose
```

**Open in Browser**:
```
http://localhost:5173
```

---

### 3. Test End-to-End Flow

1. **Login**: Click "Login with Discord"
2. **Authorize**: Grant permissions on Discord
3. **Dashboard**: View your guilds
4. **Add Streamer**: Click "Add Streamer" → Authorize Twitch
5. **Configure**: Set notification channel and template
6. **Test Notification**: Use Twitch CLI to trigger event

---

## Testing Webhooks Locally

### 1. Set Up ngrok

```bash
# Install ngrok
brew install ngrok

# Start tunnel
ngrok http 3000
```

Output:
```
Forwarding  https://abc123.ngrok.io -> http://localhost:3000
```

---

### 2. Update Webhook URLs

**Update Twitch EventSub Callback**:

In your code, temporarily override callback URL:

```go
// backend/internal/services/twitch/eventsub.go
Transport: Transport{
    Method:   "webhook",
    Callback: "https://abc123.ngrok.io/webhooks/twitch", // Use ngrok URL
    Secret:   os.Getenv("TWITCH_WEBHOOK_SECRET"),
},
```

Or set environment variable:
```bash
export WEBHOOK_BASE_URL=https://abc123.ngrok.io
```

---

### 3. Test with Twitch CLI

**Install Twitch CLI**:
```bash
brew install twitchdev/twitch/twitch-cli
```

**Configure**:
```bash
twitch configure
```

**Trigger Event**:
```bash
twitch event trigger stream.online \
  --broadcaster-user-id=123456789 \
  --forward-address=https://abc123.ngrok.io/webhooks/twitch
```

**Check Logs**:
```
2024/01/15 10:00:00 [WEBHOOK_RECEIVED] Provider: Twitch
2024/01/15 10:00:00 [FANOUT] Streamer: test_user, Guilds: 1
2024/01/15 10:00:00 [NOTIF_SENT] Guild: 123, Channel: 456
```

---

## Database Management

### View Data

```bash
# Connect to database
psql $DATABASE_URL

# List all users
SELECT * FROM users;

# List all guilds
SELECT * FROM guilds;

# List all streamers
SELECT * FROM streamers;

# View guild-streamer relationships
SELECT g.name, s.twitch_display_name
FROM guilds g
JOIN guild_streamers gs ON g.guild_id = gs.guild_id
JOIN streamers s ON gs.streamer_id = s.id;
```

---

### Seed Test Data

**File**: `backend/scripts/seed.sql`

```sql
-- Insert test user
INSERT INTO users (user_id, username, avatar)
VALUES ('123456789', 'TestUser', 'avatar_hash')
ON CONFLICT (user_id) DO NOTHING;

-- Insert test guild
INSERT INTO guilds (guild_id, name, icon)
VALUES ('987654321', 'Test Server', 'icon_hash')
ON CONFLICT (guild_id) DO NOTHING;

-- Insert test guild config
INSERT INTO guild_config (guild_id, channel_id, message_template, enabled)
VALUES ('987654321', '111222333', '{"content":"Test notification"}', true)
ON CONFLICT (guild_id) DO NOTHING;

-- Insert test streamer
INSERT INTO streamers (twitch_broadcaster_id, twitch_login, twitch_display_name)
VALUES ('555666777', 'test_streamer', 'TestStreamer')
ON CONFLICT (twitch_broadcaster_id) DO NOTHING;
```

**Apply**:
```bash
psql $DATABASE_URL -f scripts/seed.sql
```

---

### Reset Database

```bash
# Drop all tables
psql $DATABASE_URL -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Reapply migrations
for migration in migrations/*.sql; do
    psql $DATABASE_URL -f $migration
done
```

---

## Debugging

### Backend Debugging

#### With VS Code

**File**: `.vscode/launch.json`

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Backend",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/backend/cmd/lambda",
      "envFile": "${workspaceFolder}/backend/.env"
    }
  ]
}
```

**Set Breakpoints**: Click left of line numbers in VS Code

**Start Debugging**: Press F5

---

#### With Logs

Add debug logs:

```go
import "log"

log.Printf("DEBUG: Variable value: %+v", variable)
```

Set log level:
```bash
export LOG_LEVEL=debug
```

---

### Frontend Debugging

#### Browser DevTools

1. Open browser DevTools (F12)
2. **Console**: View logs
3. **Network**: View API requests
4. **Application**: View cookies, localStorage

#### React DevTools

Install React DevTools extension:
- Chrome: https://chrome.google.com/webstore/detail/react-developer-tools/fmkadmapgofadopljbjfkapdkoienihi
- Firefox: https://addons.mozilla.org/en-US/firefox/addon/react-devtools/

---

## Common Issues

### Issue: Cannot connect to database

**Error**: `connection refused`

**Solution**:
1. Verify PostgreSQL is running: `pg_isready`
2. Check DATABASE_URL is correct
3. Ensure database exists: `psql postgres -c "\l"`

---

### Issue: Go dependencies not found

**Error**: `package not found`

**Solution**:
```bash
cd backend
go mod tidy
go mod download
```

---

### Issue: Frontend build errors

**Error**: `Module not found`

**Solution**:
```bash
cd frontend
rm -rf node_modules package-lock.json
npm install
```

---

### Issue: CORS errors in browser

**Error**: `Access-Control-Allow-Origin` error

**Solution**:

Check CORS middleware in backend:

```go
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{"http://localhost:5173"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
}))
```

---

### Issue: OAuth redirect fails

**Error**: `redirect_uri_mismatch`

**Solution**:
1. Verify redirect URI in Discord/Twitch app matches exactly
2. Check `API_BASE_URL` in `.env`
3. Ensure no trailing slashes

---

### Issue: Webhook signature verification fails

**Error**: `Invalid signature`

**Solution**:
1. Verify `TWITCH_WEBHOOK_SECRET` matches in code and Twitch EventSub
2. Check timestamp is recent (< 10 minutes)
3. Ensure body is read correctly (not consumed twice)

---

## Hot Reload

### Backend (Air)

Install Air:
```bash
go install github.com/cosmtrek/air@latest
```

**File**: `backend/.air.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/main ./cmd/lambda"
bin = "tmp/main"
include_ext = ["go"]
exclude_dir = ["tmp", "vendor"]
delay = 1000
```

Run:
```bash
cd backend
air
```

---

### Frontend (Vite)

Vite has hot reload by default:
```bash
npm run dev
```

---

## Testing

### Unit Tests

**Backend**:
```bash
cd backend
go test ./...
```

**Frontend**:
```bash
cd frontend
npm test
```

---

### Integration Tests

```bash
# Start backend and frontend
# Run integration tests
cd backend
go test ./tests/integration
```

---

### E2E Tests

```bash
cd frontend
npx playwright test
```

---

## Code Quality

### Linting

**Backend**:
```bash
golangci-lint run
```

**Frontend**:
```bash
npm run lint
```

---

### Formatting

**Backend**:
```bash
go fmt ./...
```

**Frontend**:
```bash
npm run format
```

---

## Git Workflow

### Create Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### Commit Changes

```bash
git add .
git commit -m "Add feature X"
```

### Push to Remote

```bash
git push origin feature/your-feature-name
```

### Create Pull Request

1. Go to GitHub
2. Click "New Pull Request"
3. Select your branch
4. Add description
5. Create PR

---

## Checklist

- [ ] Go 1.22+ installed
- [ ] Node.js 18+ installed
- [ ] PostgreSQL installed or Neon account created
- [ ] Repository cloned
- [ ] Backend dependencies installed
- [ ] Frontend dependencies installed
- [ ] Environment variables configured
- [ ] Database created and migrations applied
- [ ] Discord application created and configured
- [ ] Twitch application created and configured
- [ ] Backend running successfully
- [ ] Frontend running successfully
- [ ] Can login with Discord
- [ ] Can add Twitch streamer
- [ ] Webhook testing working (with ngrok)

---

## Useful Commands

```bash
# Backend
go run cmd/lambda/main.go          # Run backend
go test ./...                       # Run tests
go build -o bootstrap cmd/lambda   # Build for Lambda

# Frontend
npm run dev                         # Run dev server
npm run build                       # Build for production
npm run preview                     # Preview production build

# Database
psql $DATABASE_URL                  # Connect to database
psql $DATABASE_URL -f file.sql      # Execute SQL file
pg_dump $DATABASE_URL > backup.sql  # Backup database

# Git
git status                          # Check status
git log --oneline                   # View commit history
git diff                            # View changes
```

---

## Next Steps

- Read [System/architecture.md](../System/architecture.md) for architecture overview
- Read [SOP/add-api-route.md](add-api-route.md) to learn how to add endpoints
- Read [SOP/add-db-migration.md](add-db-migration.md) to learn about migrations
