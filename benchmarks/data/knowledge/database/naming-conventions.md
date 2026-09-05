---
type: Decision
title: PostgreSQL Database Schema and Naming Conventions
description: Strict relational schema conventions requiring snake_case naming, pluralized table names, and UUIDv7 primary keys.
tags: [database, postgres, sql, conventions]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:db-architect@company.dev, at: 2026-08-15T09:00:00Z }
status: stable
---

# PostgreSQL Database Schema and Naming Conventions

All PostgreSQL database tables, columns, and indexes must strictly follow these rules:

## Naming Rules
1. **Table Names**: Lowercase, plural nouns in `snake_case` (e.g. `users`, `audit_logs`, `payment_invoices`).
2. **Primary Keys**: Always named `id` with type `uuid` generated via time-ordered **UUIDv7**. Never use serial integers.
3. **Foreign Keys**: Column name must be singular target table name plus `_id` (e.g. `organization_id` referencing `organizations(id)`).
4. **Timestamps**: All tables must include:
   - `created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()`
   - `updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()`
5. **Soft Deletes**: If soft deletion is required, use `deleted_at TIMESTAMPTZ NULL`. Never use boolean `is_deleted`.
6. **Index Naming**:
   - Primary key: `pk_<table_name>`
   - Foreign key index: `idx_<table_name>_<column_name>`
   - Unique index: `uq_<table_name>_<column_name>`

# Related Concepts
- [Redis Distributed Cache-Aside and TTL Strategy](../caching/redis-tier.md): Cache keys mirror database entity names
