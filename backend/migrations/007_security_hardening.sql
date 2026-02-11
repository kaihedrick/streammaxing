-- Migration 007: Security Hardening
-- Adds tables for session revocation, audit logging, and security events.

-- Session revocation table
-- Stores JTIs (JWT IDs) of revoked sessions so they can be rejected
-- even before their expiration time.
CREATE TABLE IF NOT EXISTS revoked_sessions (
    jti TEXT PRIMARY KEY,
    revoked_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_revoked_sessions_expires ON revoked_sessions(expires_at);

-- Audit log table
-- Tracks sensitive operations for compliance and forensics.
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ DEFAULT now(),
    user_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    details JSONB,
    ip_address TEXT,
    success BOOLEAN DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);

-- Security events table
-- Stores security events for monitoring and incident response.
CREATE TABLE IF NOT EXISTS security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ DEFAULT now(),
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    user_id TEXT,
    ip_address TEXT,
    details JSONB,
    resolved BOOLEAN DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_security_events_timestamp ON security_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_security_events_severity ON security_events(severity);
CREATE INDEX IF NOT EXISTS idx_security_events_unresolved ON security_events(resolved) WHERE resolved = false;

-- Add security tracking columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
