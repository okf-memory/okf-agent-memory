---
type: Decision
title: RFC 7807 Problem Details HTTP Error Standard
description: Standardized API error response format based on RFC 7807 problem details with typed error domains.
tags: [api, http, rest, errors, adr]
verified: { by: human:api-lead@company.dev, at: 2026-08-25T11:00:00Z }
status: stable
---

# RFC 7807 Problem Details HTTP Error Standard

All REST API endpoints must return structured errors complying with RFC 7807 (`application/problem+json`).

## JSON Payload Structure
```json
{
  "type": "https://api.company.dev/errors/auth-invalid-credentials",
  "title": "Invalid Credentials",
  "status": 401,
  "detail": "The email or password provided does not match our records.",
  "instance": "/api/v1/auth/login",
  "code": "ERR_AUTH_INVALID_CREDENTIALS",
  "timestamp": "2026-09-05T10:00:00Z"
}
```

## Mandatory Error Code Naming
Error codes must follow the exact regular expression: `^ERR_[A-Z]+_[A-Z0-9_]+$`
Examples:
- `ERR_PAYMENT_INSUFFICIENT_FUNDS`
- `ERR_VALIDATION_FIELD_REQUIRED`
- `ERR_RATE_LIMIT_EXCEEDED`
