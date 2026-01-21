---
name: testing
description: Write comprehensive tests using TDD methodology. Use for unit tests, integration tests, E2E tests with Playwright, and Go testing.
---

# Testing Skill

Test-driven development and comprehensive testing strategies for frontend and backend code.

## When to Use This Skill

- Writing unit tests for new features
- Creating E2E tests with Playwright
- Testing Go handlers and services
- Debugging test failures
- Improving test coverage

---

# 🧪 Test-Driven Development (TDD)

## The TDD Cycle

```
┌─────────────────────────────────────┐
│  1. RED: Write failing test         │
│         ↓                           │
│  2. GREEN: Write minimal code       │
│         ↓                           │
│  3. REFACTOR: Clean up code         │
│         ↓                           │
│  (Repeat)                           │
└─────────────────────────────────────┘
```

## TDD Benefits

- Forces you to think about design first
- Produces testable, modular code
- Provides instant regression safety
- Documents expected behavior

---

# 🎯 Frontend Testing

## Unit Testing (Vitest + Testing Library)

### Setup
```bash
npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom
```

### Configuration (vitest.config.ts)
```typescript
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    globals: true,
  },
});
```

### Component Test Example
```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LoginForm } from './LoginForm';

describe('LoginForm', () => {
  it('submits form with valid credentials', async () => {
    const onSubmit = vi.fn();
    render(<LoginForm onSubmit={onSubmit} />);

    await userEvent.type(screen.getByLabelText(/email/i), 'test@example.com');
    await userEvent.type(screen.getByLabelText(/password/i), 'password123');
    await userEvent.click(screen.getByRole('button', { name: /submit/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        email: 'test@example.com',
        password: 'password123',
      });
    });
  });

  it('shows validation error for invalid email', async () => {
    render(<LoginForm onSubmit={vi.fn()} />);

    await userEvent.type(screen.getByLabelText(/email/i), 'invalid-email');
    await userEvent.click(screen.getByRole('button', { name: /submit/i }));

    expect(screen.getByText(/invalid email/i)).toBeInTheDocument();
  });

  it('disables submit button while loading', () => {
    render(<LoginForm onSubmit={vi.fn()} loading />);
    expect(screen.getByRole('button', { name: /submit/i })).toBeDisabled();
  });
});
```

### Hook Testing
```tsx
import { renderHook, act } from '@testing-library/react';
import { useCounter } from './useCounter';

describe('useCounter', () => {
  it('increments counter', () => {
    const { result } = renderHook(() => useCounter());

    act(() => {
      result.current.increment();
    });

    expect(result.current.count).toBe(1);
  });
});
```

---

# 🎭 E2E Testing (Playwright)

## Setup
```bash
npm init playwright@latest
```

## Configuration (playwright.config.ts)
```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
  },
});
```

## Page Object Model
```typescript
// e2e/pages/LoginPage.ts
import { Page, Locator } from '@playwright/test';

export class LoginPage {
  readonly page: Page;
  readonly emailInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page) {
    this.page = page;
    this.emailInput = page.getByLabel('Email');
    this.passwordInput = page.getByLabel('Password');
    this.submitButton = page.getByRole('button', { name: 'Sign in' });
    this.errorMessage = page.getByRole('alert');
  }

  async goto() {
    await this.page.goto('/login');
  }

  async login(email: string, password: string) {
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }
}
```

## E2E Test Example
```typescript
import { test, expect } from '@playwright/test';
import { LoginPage } from './pages/LoginPage';

test.describe('Authentication', () => {
  test('successful login redirects to dashboard', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login('user@example.com', 'validpassword');

    await expect(page).toHaveURL('/dashboard');
    await expect(page.getByText('Welcome')).toBeVisible();
  });

  test('invalid credentials show error', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login('user@example.com', 'wrongpassword');

    await expect(loginPage.errorMessage).toHaveText('Invalid credentials');
    await expect(page).toHaveURL('/login');
  });
});
```

## API Testing with Playwright
```typescript
import { test, expect } from '@playwright/test';

test.describe('API Tests', () => {
  test('GET /api/users returns user list', async ({ request }) => {
    const response = await request.get('/api/users');
    
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data.users).toBeInstanceOf(Array);
  });

  test('POST /api/users creates user', async ({ request }) => {
    const response = await request.post('/api/users', {
      data: { name: 'Test User', email: 'test@example.com' },
    });

    expect(response.status()).toBe(201);
    const user = await response.json();
    expect(user.id).toBeDefined();
  });
});
```

---

# 🔧 Go Testing

## Table-Driven Tests
```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {
            name:  "valid config",
            input: `{"port": 8080}`,
            want:  &Config{Port: 8080},
        },
        {
            name:    "invalid json",
            input:   `{invalid}`,
            wantErr: true,
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## HTTP Handler Testing
```go
func TestHandler_GetUser(t *testing.T) {
    // Setup
    mockStore := &MockUserStore{
        users: map[string]*User{
            "123": {ID: "123", Name: "Test User"},
        },
    }
    handler := NewHandler(mockStore)

    // Create request
    req := httptest.NewRequest("GET", "/users/123", nil)
    req = mux.SetURLVars(req, map[string]string{"id": "123"})
    w := httptest.NewRecorder()

    // Execute
    handler.GetUser(w, req)

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)
    
    var response User
    err := json.Unmarshal(w.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Equal(t, "Test User", response.Name)
}
```

## Mocking with Interfaces
```go
// Define interface
type UserStore interface {
    GetUser(id string) (*User, error)
    CreateUser(user *User) error
}

// Mock implementation
type MockUserStore struct {
    users map[string]*User
    err   error
}

func (m *MockUserStore) GetUser(id string) (*User, error) {
    if m.err != nil {
        return nil, m.err
    }
    user, ok := m.users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}
```

## Integration Tests
```go
//go:build integration

func TestDatabase_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    db := setupTestDB(t)
    defer db.Close()

    // Test actual database operations
    user := &User{Name: "Test"}
    err := db.CreateUser(user)
    assert.NoError(t, err)
    assert.NotEmpty(t, user.ID)
}
```

---

# ❌ Testing Anti-Patterns

## Avoid These

1. **Testing implementation, not behavior**
```go
// ❌ Bad: Testing internal state
assert.Equal(t, 3, len(cache.items))

// ✅ Good: Testing behavior
result, err := cache.Get("key")
assert.NoError(t, err)
assert.Equal(t, expectedValue, result)
```

2. **Flaky tests**
```typescript
// ❌ Bad: Time-dependent
expect(Date.now() - startTime).toBeLessThan(100);

// ✅ Good: Mock time
vi.useFakeTimers();
vi.setSystemTime(new Date('2024-01-01'));
```

3. **Tests that don't clean up**
```go
// ✅ Good: Always cleanup
func TestWithTempFile(t *testing.T) {
    f, err := os.CreateTemp("", "test")
    require.NoError(t, err)
    defer os.Remove(f.Name())  // Cleanup
    
    // Test logic
}
```

---

# 📚 References

- [anthropics/webapp-testing](https://github.com/anthropics/skills/tree/main/skills/webapp-testing)
- [obra/test-driven-development](https://github.com/obra/superpowers/blob/main/skills/test-driven-development/SKILL.md)
- [trailofbits/property-based-testing](https://github.com/trailofbits/skills/tree/main/plugins/property-based-testing)
- [lackeyjb/playwright-skill](https://github.com/lackeyjb/playwright-skill)
- [Playwright Documentation](https://playwright.dev/docs/intro)
