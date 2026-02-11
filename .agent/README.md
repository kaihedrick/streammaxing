# StreamMaxing v3 - Documentation Index

## Project Overview

StreamMaxing v3 is a serverless Twitch → Discord notification system that sends real-time notifications to Discord servers when Twitch streamers go live.

### Key Features
- **Multi-Server Support**: Unlimited Discord servers can use the same bot
- **Unlimited Streamers**: Track any number of Twitch streamers (cost permitting)
- **User Preferences**: Per-user notification settings (mute specific streamers)
- **Advanced Templates**: Rich Discord embeds with thumbnails, fields, and custom formatting
- **Serverless Architecture**: AWS Lambda + Neon Postgres for minimal cost (<$20/month)
- **Real-Time Notifications**: Powered by Twitch EventSub webhooks

### Architecture Summary
- **Backend**: Go 1.22+ on AWS Lambda
- **Frontend**: React + TypeScript (Vite) on S3 + CloudFront
- **Database**: Neon Postgres (serverless)
- **Auth**: Discord OAuth + Twitch OAuth
- **Events**: Twitch EventSub webhooks
- **Notifications**: Discord Bot API
- **Config**: Centralized config package (`internal/config/`) — all secrets loaded from AWS Secrets Manager (prod) or env vars (dev); no `os.Getenv` scattered across services

### Cost Target
Less than $20/month for multi-server support with reasonable usage (10-50 streamers, 5-10 servers).

---

## Documentation Structure

### System Documentation
Core system architecture, integrations, and technical specifications.

1. **[architecture.md](System/architecture.md)** - High-level system architecture, event flows, CloudFront setup
2. **[tech-stack.md](System/tech-stack.md)** - Technology choices, versions, and requirements
3. **[auth.md](System/auth.md)** - OAuth flows for Discord and Twitch, session management
4. **[integrations.md](System/integrations.md)** - Twitch EventSub and Discord integration details
5. **[database.md](System/database.md)** - Complete database schema, tables, relationships
6. **[cost-model.md](System/cost-model.md)** - Cost breakdown, monitoring, and cost guards

### Task Documentation
PRD and implementation plans for each feature.

1. **[001-project-bootstrap.md](Tasks/001-project-bootstrap.md)** - Initial project setup
2. **[002-discord-auth-and-bot-install.md](Tasks/002-discord-auth-and-bot-install.md)** - Discord OAuth and bot installation
3. **[003-twitch-auth-and-eventsub.md](Tasks/003-twitch-auth-and-eventsub.md)** - Twitch integration and EventSub
4. **[004-notification-fanout.md](Tasks/004-notification-fanout.md)** - Notification delivery with user preferences
5. **[005-admin-dashboard.md](Tasks/005-admin-dashboard.md)** - Admin UI and user settings
6. **[006-hardening-and-cleanup.md](Tasks/006-hardening-and-cleanup.md)** - Edge cases and cleanup logic
7. **[007-security-hardening.md](Tasks/007-security-hardening.md)** - Security hardening and production readiness (IMPLEMENTED)

### Standard Operating Procedures
Best practices for common development tasks.

1. **[add-api-route.md](SOP/add-api-route.md)** - How to add new API endpoints
2. **[add-db-migration.md](SOP/add-db-migration.md)** - How to create and apply database migrations
3. **[add-new-oauth-provider.md](SOP/add-new-oauth-provider.md)** - How to add OAuth providers
4. **[handle-webhooks.md](SOP/handle-webhooks.md)** - How to handle webhook events
5. **[local-development.md](SOP/local-development.md)** - Local development setup guide
6. **[deployment.md](SOP/deployment.md)** - Deployment procedures for AWS infrastructure
7. **[security-best-practices.md](SOP/security-best-practices.md)** - Security guidelines and best practices

---

## Quick Start

1. Read [System/architecture.md](System/architecture.md) for overall system design
2. Read [System/tech-stack.md](System/tech-stack.md) for technology requirements
3. Follow [SOP/local-development.md](SOP/local-development.md) to set up your dev environment
4. Review task documentation in order (001 through 006) for feature implementation details

---

## Maintenance Guidelines

This documentation should be updated whenever:
- A new feature is implemented
- The database schema changes
- Integration points are modified
- New edge cases are discovered
- Cost structure changes

Always keep this documentation as the single source of truth for the project.
