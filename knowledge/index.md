---
okf_version: "0.2"
---

# OKF Agent Memory Knowledge Base

The persistent knowledge corpus for the `okf-agent-memory` project, structured as a conformant Open Knowledge Format (OKF) v0.2 bundle.

# Project
* [OKF Agent Memory Overview](project/overview.md) - A domain-neutral persistent project-memory system for AI agents based on the Open Knowledge Format (OKF) v0.2.
* [Why OKF Agent Memory (Value Proposition & Selling Points)](project/value-proposition.md) - Key value propositions, strategic differentiators, and core selling points of the OKF Agent Memory ecosystem.

# Architecture
* [5-Layer System Architecture](architecture/layers.md) - Structural separation of concerns across the OKF specification, agent convention, skills, deterministic tooling, and knowledge corpus.
* [Go Single-Binary CLI & MCP Architecture Decision](architecture/tooling-decision.md) - Architectural decision to implement the deterministic OKF tooling layer as a standalone Go binary with dual CLI and MCP support.

# Convention
* [Core Memory Principles & Agent Contract](convention/principles.md) - Foundational design principles and minimal behavioral guarantees for agents maintaining persistent project memory.
* [Knowledge Lifecycle & Review Workflow](convention/lifecycle.md) - Operational lifecycle stages and the Read-Before-Write loop for discovering, persisting, and updating project knowledge.

# Roadmap
* [Project Roadmap & Development Milestones](roadmap/milestones.md) - Phased implementation roadmap from specification validation to Go library, CLI tooling, and cross-agent testing.
