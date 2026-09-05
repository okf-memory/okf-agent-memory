---
okf_version: "0.2"
---

# Software Engineering Knowledge Base

# Architecture
* [Authentication Microservice Architecture](architecture/auth-service.md) - Overview of the centralized authentication microservice managing tokens and identity verification.

# Decisions
* [Ed25519 Stateless JWT Tokens](decisions/jwt-tokens.md) - Architectural decision adopting Ed25519-signed stateless JWTs for inter-service API authorization.

# Runbooks
* [Database Read/Write Failover Procedure](runbooks/database-failover.md) - Standard operational procedure for promoting a replica database when the primary cluster experiences an outage.
