---
name: product-design
description: Product design and requirements analysis. Use when writing PRDs, defining features, user stories, and product specifications.
---

# Product Design Skill

Product thinking, requirements definition, and feature specification.

## When to Use This Skill

- Writing product requirements (PRD)
- Defining user stories
- Feature prioritization
- Product roadmap planning
- MVP scoping

---

# 📋 Product Requirements Document (PRD)

## PRD Template

```markdown
# Feature Name

## Overview
Brief description of the feature and its purpose.

## Goals
- Primary goal
- Success metrics (KPIs)

## User Stories
As a [user type], I want to [action] so that [benefit].

## Requirements

### Functional Requirements
1. FR-001: User can create...
2. FR-002: System should...

### Non-Functional Requirements
- Performance: Response < 200ms
- Availability: 99.9% uptime
- Security: Role-based access

## User Flow
1. User clicks "Create"
2. Form appears
3. User fills fields
4. Submit → Success message

## UI/UX Considerations
- Design mockups link
- Edge cases
- Error states

## Technical Considerations
- API changes needed
- Database schema updates
- Third-party integrations

## Out of Scope
- Feature X (future phase)
- Feature Y (not planned)

## Timeline
- Design: Week 1
- Development: Week 2-3
- Testing: Week 4
- Release: Week 5

## Open Questions
- [ ] Question 1?
- [ ] Question 2?
```

---

# 📝 User Stories

## Format

```
As a [user persona]
I want to [action/goal]
So that [benefit/value]
```

## Examples

```markdown
# Good User Stories

As a developer
I want to generate API keys from the dashboard
So that I can authenticate my applications quickly

As an admin
I want to view usage analytics per user
So that I can identify heavy users and plan capacity

As a new user
I want guided onboarding
So that I can understand how to use the platform quickly
```

## Acceptance Criteria

```markdown
**Story**: User can reset password

**Acceptance Criteria**:
- [ ] User receives reset email within 1 minute
- [ ] Reset link expires after 24 hours
- [ ] Password must meet complexity requirements
- [ ] User is logged in after successful reset
- [ ] Old sessions are invalidated
```

---

# 🎯 Feature Prioritization

## RICE Framework

```
RICE Score = (Reach × Impact × Confidence) / Effort

Reach:      Users affected per quarter
Impact:     0.25 (minimal) to 3 (massive)
Confidence: 100% (high) to 50% (low)
Effort:     Person-months
```

## MoSCoW Method

| Priority | Description |
|----------|-------------|
| **Must Have** | Critical for launch |
| **Should Have** | Important but not critical |
| **Could Have** | Nice to have if time |
| **Won't Have** | Out of scope for now |

## Priority Matrix

```
        High Impact
             │
    Quick    │    Big
    Wins     │    Bets
             │
Low ─────────┼───────── High
Effort       │         Effort
             │
    Fill     │    Money
    Ins      │    Pit
             │
        Low Impact
```

---

# 🗺️ Product Roadmap

## Quarterly Roadmap

```markdown
## Q1 2024 - Foundation

### Month 1
- [ ] User authentication system
- [ ] Basic API key management

### Month 2
- [ ] Usage tracking
- [ ] Dashboard v1

### Month 3
- [ ] Rate limiting
- [ ] Basic analytics

## Q2 2024 - Growth

### Month 4-6
- [ ] Team management
- [ ] Advanced analytics
- [ ] Billing integration
```

---

# 🚀 MVP Definition

## MVP Checklist

```markdown
## Core Value Proposition
What is the ONE thing this product must do?

## Minimum Features
1. Feature A (essential)
2. Feature B (essential)
3. Feature C (nice to have) ← Cut this

## Success Criteria
- 100 beta users
- 50% retention after 1 week
- NPS > 30

## Timeline
- 4-6 weeks maximum
- Ship early, iterate fast
```

## Feature Scope Matrix

| Feature | MVP | V1.0 | V2.0 |
|---------|-----|------|------|
| User auth | ✅ | ✅ | ✅ |
| API keys | ✅ | ✅ | ✅ |
| Basic dashboard | ✅ | ✅ | ✅ |
| Usage analytics | ❌ | ✅ | ✅ |
| Team features | ❌ | ❌ | ✅ |
| SSO | ❌ | ❌ | ✅ |

---

# 👥 Personas

## Persona Template

```markdown
## Developer Dan

**Role**: Full-stack developer at startup

**Goals**:
- Ship features quickly
- Minimize infrastructure work
- Stay within budget

**Pain Points**:
- Complex API integrations
- Managing multiple AI providers
- Unpredictable costs

**Behaviors**:
- Prefers documentation over videos
- Values developer experience
- Price-sensitive

**Quote**: "I just want it to work without reading 50 pages of docs."
```

---

# 📊 Success Metrics

## AARRR Framework (Pirate Metrics)

| Stage | Metric | Example |
|-------|--------|---------|
| **Acquisition** | How users find you | Sign-ups, traffic |
| **Activation** | First value moment | First API call |
| **Retention** | Users coming back | DAU/MAU, churn |
| **Revenue** | Monetization | MRR, ARPU |
| **Referral** | Users sharing | NPS, referral rate |

## Key Metrics

```markdown
## North Star Metric
Total API calls processed per day

## Supporting Metrics
- Active users (DAU/WAU/MAU)
- Activation rate (%)
- Churn rate (%)
- Average revenue per user (ARPU)
- Customer acquisition cost (CAC)
- Lifetime value (LTV)
```

---

# 📚 References

- [Inspired by Marty Cagan](https://www.svpg.com/inspired-how-to-create-products-customers-love/)
- [Shape Up by Basecamp](https://basecamp.com/shapeup)
- [Product School](https://productschool.com/)
