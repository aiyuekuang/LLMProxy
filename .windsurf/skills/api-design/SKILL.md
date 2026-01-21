---
name: api-design
description: RESTful API design best practices. Use when designing endpoints, request/response schemas, error handling, and API versioning.
---

# API Design Skill

RESTful API design principles and patterns for building robust, scalable APIs.

## When to Use This Skill

- Designing new API endpoints
- Defining request/response schemas
- Implementing error handling
- API versioning strategies
- OpenAPI/Swagger documentation

---

# 🔗 RESTful Conventions

## Resource Naming

```
# ✅ Good: Plural nouns, lowercase, hyphens
GET    /api/v1/users
GET    /api/v1/users/{id}
POST   /api/v1/users
PUT    /api/v1/users/{id}
DELETE /api/v1/users/{id}

GET    /api/v1/api-keys
GET    /api/v1/model-providers

# ❌ Bad: Verbs, camelCase, underscores
GET    /api/v1/getUsers
POST   /api/v1/createUser
GET    /api/v1/api_keys
```

## Nested Resources

```
# User's API keys
GET    /api/v1/users/{userId}/api-keys
POST   /api/v1/users/{userId}/api-keys

# Model's versions
GET    /api/v1/models/{modelId}/versions
```

## HTTP Methods

| Method | Usage | Idempotent |
|--------|-------|------------|
| GET | Retrieve resource(s) | Yes |
| POST | Create resource | No |
| PUT | Replace resource | Yes |
| PATCH | Partial update | Yes |
| DELETE | Remove resource | Yes |

---

# 📦 Request/Response Format

## Successful Response

```json
{
  "success": true,
  "data": {
    "id": "user_123",
    "name": "John Doe",
    "email": "john@example.com",
    "createdAt": "2024-01-15T10:30:00Z"
  }
}
```

## List Response with Pagination

```json
{
  "success": true,
  "data": [
    { "id": "1", "name": "Item 1" },
    { "id": "2", "name": "Item 2" }
  ],
  "meta": {
    "page": 1,
    "pageSize": 20,
    "total": 100,
    "totalPages": 5
  }
}
```

## Error Response

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": [
      {
        "field": "email",
        "message": "Invalid email format"
      },
      {
        "field": "password",
        "message": "Password must be at least 8 characters"
      }
    ]
  },
  "requestId": "req_abc123"
}
```

---

# 📊 HTTP Status Codes

## Success (2xx)

| Code | Usage |
|------|-------|
| 200 OK | General success, GET/PUT/PATCH |
| 201 Created | Resource created (POST) |
| 204 No Content | Success with no body (DELETE) |

## Client Errors (4xx)

| Code | Usage |
|------|-------|
| 400 Bad Request | Invalid request format/data |
| 401 Unauthorized | Missing/invalid authentication |
| 403 Forbidden | Authenticated but not authorized |
| 404 Not Found | Resource doesn't exist |
| 409 Conflict | Resource conflict (duplicate) |
| 422 Unprocessable | Validation errors |
| 429 Too Many Requests | Rate limit exceeded |

## Server Errors (5xx)

| Code | Usage |
|------|-------|
| 500 Internal Server Error | Unexpected server error |
| 502 Bad Gateway | Upstream service error |
| 503 Service Unavailable | Service temporarily down |
| 504 Gateway Timeout | Upstream timeout |

---

# 🔍 Query Parameters

## Filtering

```
GET /api/v1/users?status=active&role=admin
GET /api/v1/models?provider=openai&type=chat
```

## Sorting

```
GET /api/v1/users?sort=createdAt:desc
GET /api/v1/users?sort=name:asc,createdAt:desc
```

## Pagination

```
# Offset-based (simple but slower for large datasets)
GET /api/v1/users?page=2&pageSize=20

# Cursor-based (better for large datasets)
GET /api/v1/users?cursor=eyJpZCI6MTAwfQ&limit=20
```

## Field Selection

```
GET /api/v1/users?fields=id,name,email
GET /api/v1/users?include=profile,settings
```

## Search

```
GET /api/v1/users?q=john
GET /api/v1/users?search=john@example
```

---

# 🔐 Authentication

## API Key (Header)

```
GET /api/v1/models
Authorization: Bearer sk-xxxxxxxxxxxxx
```

## API Key (Query - less secure)

```
GET /api/v1/models?api_key=sk-xxxxxxxxxxxxx
```

## JWT Token

```
GET /api/v1/users/me
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

---

# 📋 API Versioning

## URL Path (Recommended)

```
/api/v1/users
/api/v2/users
```

## Header

```
GET /api/users
API-Version: 2024-01-15
```

## Query Parameter

```
GET /api/users?version=2
```

---

# 📖 OpenAPI Specification

```yaml
openapi: 3.0.3
info:
  title: LLMProxy API
  version: 1.0.0
  description: API for managing LLM proxy services

servers:
  - url: https://api.llmproxy.io/v1

paths:
  /models:
    get:
      summary: List all models
      tags: [Models]
      parameters:
        - name: provider
          in: query
          schema:
            type: string
            enum: [openai, anthropic, azure]
        - name: page
          in: query
          schema:
            type: integer
            default: 1
      responses:
        '200':
          description: List of models
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ModelListResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'

components:
  schemas:
    Model:
      type: object
      required: [id, name, provider]
      properties:
        id:
          type: string
          example: "model_123"
        name:
          type: string
          example: "gpt-4"
        provider:
          type: string
          enum: [openai, anthropic, azure]
        
    ModelListResponse:
      type: object
      properties:
        success:
          type: boolean
        data:
          type: array
          items:
            $ref: '#/components/schemas/Model'
        meta:
          $ref: '#/components/schemas/PaginationMeta'

  responses:
    Unauthorized:
      description: Authentication required
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'

  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
```

---

# ⚡ Rate Limiting Headers

```
HTTP/1.1 200 OK
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
Retry-After: 60
```

---

# 📚 References

- [Microsoft REST API Guidelines](https://github.com/microsoft/api-guidelines)
- [Google API Design Guide](https://cloud.google.com/apis/design)
- [JSON:API Specification](https://jsonapi.org/)
- [OpenAPI Specification](https://swagger.io/specification/)
