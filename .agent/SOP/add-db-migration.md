# SOP: Add Database Migration

## Overview
This document describes the procedure for creating and applying database migrations in StreamMaxing.

---

## Prerequisites
- Database connection configured
- `psql` or similar PostgreSQL client installed
- Access to Neon dashboard (for production migrations)

---

## Migration Naming Convention

Format: `XXX_description.sql`

Examples:
- `001_initial_schema.sql`
- `002_add_user_preferences.sql`
- `003_add_notification_log_index.sql`

---

## Steps

### 1. Create Migration File

**Location**: `backend/migrations/`

**File**: `backend/migrations/003_add_example_table.sql`

```sql
-- Migration: Add example table
-- Date: 2024-01-15
-- Description: Create example table for storing example data

BEGIN;

-- Create table
CREATE TABLE IF NOT EXISTS example_table (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    value TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Add indexes
CREATE INDEX idx_example_table_user ON example_table(user_id);
CREATE INDEX idx_example_table_name ON example_table(name);

-- Add constraints
ALTER TABLE example_table
    ADD CONSTRAINT example_table_name_length CHECK (length(name) <= 100);

COMMIT;
```

---

### 2. Create Rollback Migration (Optional)

**File**: `backend/migrations/003_add_example_table.down.sql`

```sql
-- Rollback Migration: Remove example table
-- Date: 2024-01-15

BEGIN;

-- Drop table (CASCADE will drop dependent objects)
DROP TABLE IF EXISTS example_table CASCADE;

COMMIT;
```

---

### 3. Test Migration Locally

#### Using psql

```bash
# Apply migration
psql $DATABASE_URL -f migrations/003_add_example_table.sql

# Verify table created
psql $DATABASE_URL -c "\d example_table"

# Test rollback (optional)
psql $DATABASE_URL -f migrations/003_add_example_table.down.sql
```

#### Using Go Script

**File**: `backend/scripts/migrate.go`

```go
package main

import (
    "context"
    "fmt"
    "io/ioutil"
    "log"
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: go run scripts/migrate.go <migration_file>")
    }

    migrationFile := os.Args[1]

    // Connect to database
    pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer pool.Close()

    // Read migration file
    sql, err := ioutil.ReadFile(migrationFile)
    if err != nil {
        log.Fatalf("Failed to read migration: %v", err)
    }

    // Execute migration
    _, err = pool.Exec(context.Background(), string(sql))
    if err != nil {
        log.Fatalf("Migration failed: %v", err)
    }

    fmt.Println("Migration applied successfully")
}
```

**Usage**:
```bash
go run scripts/migrate.go migrations/003_add_example_table.sql
```

---

### 4. Update Go Models (if needed)

If migration adds/modifies tables, update corresponding Go structs.

**File**: `backend/internal/db/example.go`

```go
package db

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type ExampleDB struct {
    pool *pgxpool.Pool
}

func NewExampleDB(pool *pgxpool.Pool) *ExampleDB {
    return &ExampleDB{pool: pool}
}

type Example struct {
    ID        uuid.UUID `json:"id"`
    UserID    string    `json:"user_id"`
    Name      string    `json:"name"`
    Value     string    `json:"value"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (db *ExampleDB) Create(userID, name, value string) (*Example, error) {
    query := `
        INSERT INTO example_table (user_id, name, value)
        VALUES ($1, $2, $3)
        RETURNING id, user_id, name, value, created_at, updated_at
    `

    var example Example
    err := db.pool.QueryRow(context.Background(), query, userID, name, value).Scan(
        &example.ID,
        &example.UserID,
        &example.Name,
        &example.Value,
        &example.CreatedAt,
        &example.UpdatedAt,
    )

    return &example, err
}
```

---

### 5. Apply to Production

#### Option A: Manual (Neon Dashboard)

1. Log in to Neon dashboard
2. Navigate to your database
3. Open SQL Editor
4. Copy migration SQL
5. Execute
6. Verify schema changes

#### Option B: CLI (psql)

```bash
# Set production DATABASE_URL
export DATABASE_URL="postgresql://user:pass@prod.neon.tech/db"

# Apply migration
psql $DATABASE_URL -f migrations/003_add_example_table.sql

# Verify
psql $DATABASE_URL -c "\d example_table"
```

#### Option C: Automated Script

**File**: `backend/scripts/apply-migrations.sh`

```bash
#!/bin/bash

# Apply all pending migrations

MIGRATIONS_DIR="migrations"
DATABASE_URL="${DATABASE_URL}"

if [ -z "$DATABASE_URL" ]; then
    echo "Error: DATABASE_URL not set"
    exit 1
fi

# Loop through migration files in order
for migration in $(ls $MIGRATIONS_DIR/*.sql | sort); do
    echo "Applying $migration..."
    psql $DATABASE_URL -f $migration

    if [ $? -ne 0 ]; then
        echo "Error applying $migration"
        exit 1
    fi
done

echo "All migrations applied successfully"
```

**Usage**:
```bash
chmod +x scripts/apply-migrations.sh
./scripts/apply-migrations.sh
```

---

### 6. Track Applied Migrations (Advanced)

Create a migration tracking table:

**File**: `backend/migrations/000_migration_tracking.sql`

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    id SERIAL PRIMARY KEY,
    version TEXT UNIQUE NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT now()
);
```

**Update Migration Script**:

```go
func applyMigration(pool *pgxpool.Pool, migrationFile string) error {
    // Extract version from filename
    version := filepath.Base(migrationFile)

    // Check if already applied
    var exists bool
    err := pool.QueryRow(context.Background(),
        "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
        version,
    ).Scan(&exists)

    if err != nil {
        return err
    }

    if exists {
        fmt.Printf("Migration %s already applied, skipping\n", version)
        return nil
    }

    // Read and execute migration
    sql, err := ioutil.ReadFile(migrationFile)
    if err != nil {
        return err
    }

    _, err = pool.Exec(context.Background(), string(sql))
    if err != nil {
        return err
    }

    // Record migration
    _, err = pool.Exec(context.Background(),
        "INSERT INTO schema_migrations (version) VALUES ($1)",
        version,
    )

    return err
}
```

---

## Migration Best Practices

### 1. Use Transactions

Always wrap migrations in `BEGIN`/`COMMIT`:

```sql
BEGIN;

-- Migration statements here

COMMIT;
```

If any statement fails, entire migration rolls back.

---

### 2. Check Existence Before Creating

```sql
CREATE TABLE IF NOT EXISTS table_name (...);
CREATE INDEX IF NOT EXISTS idx_name ON table_name(column);
```

This makes migrations idempotent (safe to run multiple times).

---

### 3. Add Constraints Carefully

Test constraints on existing data:

```sql
-- Check constraint
ALTER TABLE users
    ADD CONSTRAINT username_not_empty CHECK (username != '');

-- Foreign key
ALTER TABLE user_preferences
    ADD CONSTRAINT fk_user
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE;
```

---

### 4. Use CASCADE for Foreign Keys

```sql
FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
```

This ensures cleanup when parent records are deleted.

---

### 5. Add Indexes for Performance

```sql
-- Single column index
CREATE INDEX idx_users_email ON users(email);

-- Composite index
CREATE INDEX idx_guild_streamers_lookup ON guild_streamers(guild_id, streamer_id);

-- Unique index
CREATE UNIQUE INDEX idx_streamers_broadcaster ON streamers(twitch_broadcaster_id);
```

---

### 6. Set Default Values

```sql
ALTER TABLE guilds
    ALTER COLUMN enabled SET DEFAULT true;

ALTER TABLE users
    ALTER COLUMN created_at SET DEFAULT now();
```

---

### 7. Use Enums Carefully

```sql
-- Create enum type
CREATE TYPE subscription_status AS ENUM ('pending', 'enabled', 'failed');

-- Use in table
CREATE TABLE eventsub_subscriptions (
    id UUID PRIMARY KEY,
    status subscription_status NOT NULL DEFAULT 'pending'
);
```

**Note**: Enums are hard to modify. Consider using TEXT with CHECK constraint instead:

```sql
CREATE TABLE eventsub_subscriptions (
    id UUID PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'pending',
    CONSTRAINT status_check CHECK (status IN ('pending', 'enabled', 'failed'))
);
```

---

## Common Migration Patterns

### Add Column

```sql
ALTER TABLE users
    ADD COLUMN last_login TIMESTAMPTZ DEFAULT now();
```

---

### Rename Column

```sql
ALTER TABLE users
    RENAME COLUMN username TO display_name;
```

---

### Change Column Type

```sql
-- Must cast if data exists
ALTER TABLE guilds
    ALTER COLUMN owner_id TYPE BIGINT USING owner_id::BIGINT;
```

---

### Drop Column

```sql
ALTER TABLE users
    DROP COLUMN IF EXISTS old_column;
```

---

### Add Index

```sql
CREATE INDEX CONCURRENTLY idx_users_created_at ON users(created_at);
```

**Note**: `CONCURRENTLY` prevents table locking (slower but safer for production).

---

### Drop Index

```sql
DROP INDEX IF EXISTS idx_old_index;
```

---

### Rename Table

```sql
ALTER TABLE old_table_name RENAME TO new_table_name;
```

---

## Rollback Procedure

### 1. Identify Problem

```bash
# Check recent migrations
psql $DATABASE_URL -c "SELECT * FROM schema_migrations ORDER BY applied_at DESC LIMIT 5;"
```

---

### 2. Apply Rollback Migration

```bash
psql $DATABASE_URL -f migrations/003_add_example_table.down.sql
```

---

### 3. Remove Migration Record

```sql
DELETE FROM schema_migrations WHERE version = '003_add_example_table.sql';
```

---

## Testing Migrations

### 1. Test on Copy of Production Data

```bash
# Export production data
pg_dump $PROD_DATABASE_URL > production_dump.sql

# Import to local database
psql $LOCAL_DATABASE_URL < production_dump.sql

# Test migration
psql $LOCAL_DATABASE_URL -f migrations/003_new_migration.sql
```

---

### 2. Test Rollback

```bash
psql $LOCAL_DATABASE_URL -f migrations/003_new_migration.down.sql
```

---

### 3. Verify Data Integrity

```sql
-- Check row counts
SELECT COUNT(*) FROM table_name;

-- Check constraints
SELECT * FROM information_schema.table_constraints WHERE table_name = 'table_name';

-- Check indexes
SELECT * FROM pg_indexes WHERE tablename = 'table_name';
```

---

## Troubleshooting

### Issue: Migration Fails Midway

**Cause**: Statement error in migration
**Fix**:
1. Rollback transaction (automatic if using BEGIN/COMMIT)
2. Fix migration file
3. Reapply

---

### Issue: Cannot Add NOT NULL Constraint

**Cause**: Existing NULL values in column
**Fix**:
```sql
-- First, update NULL values
UPDATE table_name SET column_name = 'default_value' WHERE column_name IS NULL;

-- Then add constraint
ALTER TABLE table_name ALTER COLUMN column_name SET NOT NULL;
```

---

### Issue: Foreign Key Constraint Fails

**Cause**: Orphaned records
**Fix**:
```sql
-- Find orphaned records
SELECT * FROM child_table
WHERE parent_id NOT IN (SELECT id FROM parent_table);

-- Delete orphaned records
DELETE FROM child_table
WHERE parent_id NOT IN (SELECT id FROM parent_table);

-- Then add foreign key
ALTER TABLE child_table
    ADD CONSTRAINT fk_parent FOREIGN KEY (parent_id) REFERENCES parent_table(id);
```

---

## Checklist

- [ ] Migration file created with descriptive name
- [ ] Rollback migration created (optional)
- [ ] Transaction wrapping used (BEGIN/COMMIT)
- [ ] IF NOT EXISTS checks added
- [ ] Indexes created for foreign keys
- [ ] Default values set appropriately
- [ ] Tested on local database
- [ ] Tested on copy of production data
- [ ] Go models updated (if needed)
- [ ] Documentation updated
- [ ] Applied to production
- [ ] Verified in production
- [ ] Migration tracked in schema_migrations table

---

## Related Documentation

- [System/database.md](../System/database.md) - Full database schema
- [SOP/local-development.md](local-development.md) - Local database setup
