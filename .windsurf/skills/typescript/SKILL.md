---
name: typescript
description: TypeScript best practices and patterns. Use for type definitions, generics, utility types, and advanced TypeScript features.
---

# TypeScript Skill

TypeScript patterns, type safety, and best practices for frontend and backend development.

## When to Use This Skill

- Defining types and interfaces
- Using generics effectively
- Type narrowing and guards
- Utility types
- Error handling with types

---

# 📐 Type Definitions

## Interface vs Type

```typescript
// Interface: Prefer for object shapes, can be extended
interface User {
  id: string;
  name: string;
  email: string;
}

interface AdminUser extends User {
  role: 'admin';
  permissions: string[];
}

// Type: Prefer for unions, tuples, primitives
type Status = 'pending' | 'active' | 'inactive';
type Result<T> = { success: true; data: T } | { success: false; error: string };
type Coordinates = [number, number];
```

## Strict Typing

```typescript
// ✅ Good: Explicit types
interface ApiResponse<T> {
  success: boolean;
  data: T;
  meta: {
    page: number;
    total: number;
  };
}

// ✅ Good: Const assertions for literals
const CONFIG = {
  apiUrl: 'https://api.example.com',
  timeout: 5000,
} as const;

// ❌ Bad: Using `any`
function processData(data: any) { }

// ✅ Good: Use `unknown` and type guards
function processData(data: unknown) {
  if (isValidData(data)) {
    // data is now typed
  }
}
```

---

# 🔧 Generics

## Basic Generics

```typescript
// Generic function
function first<T>(arr: T[]): T | undefined {
  return arr[0];
}

// Generic interface
interface Repository<T> {
  findById(id: string): Promise<T | null>;
  findAll(): Promise<T[]>;
  create(item: Omit<T, 'id'>): Promise<T>;
  update(id: string, item: Partial<T>): Promise<T>;
  delete(id: string): Promise<void>;
}

// Generic class
class ApiClient<T> {
  constructor(private baseUrl: string) {}

  async get(endpoint: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}${endpoint}`);
    return res.json();
  }
}
```

## Constraints

```typescript
// Constrain to specific shape
interface HasId {
  id: string;
}

function findById<T extends HasId>(items: T[], id: string): T | undefined {
  return items.find(item => item.id === id);
}

// Multiple constraints
function merge<T extends object, U extends object>(a: T, b: U): T & U {
  return { ...a, ...b };
}
```

## Generic Utilities

```typescript
// Pick specific keys
type UserPreview = Pick<User, 'id' | 'name'>;

// Omit specific keys
type CreateUserInput = Omit<User, 'id' | 'createdAt'>;

// Make all properties optional
type PartialUser = Partial<User>;

// Make all properties required
type RequiredUser = Required<User>;

// Make all properties readonly
type ReadonlyUser = Readonly<User>;

// Record type
type UserRoles = Record<string, 'admin' | 'user' | 'guest'>;
```

---

# 🛡️ Type Guards

## Type Predicates

```typescript
interface Cat {
  type: 'cat';
  meow(): void;
}

interface Dog {
  type: 'dog';
  bark(): void;
}

type Animal = Cat | Dog;

// Type predicate
function isCat(animal: Animal): animal is Cat {
  return animal.type === 'cat';
}

function handleAnimal(animal: Animal) {
  if (isCat(animal)) {
    animal.meow();  // TypeScript knows it's Cat
  } else {
    animal.bark();  // TypeScript knows it's Dog
  }
}
```

## Discriminated Unions

```typescript
type Result<T, E = Error> = 
  | { success: true; data: T }
  | { success: false; error: E };

function handleResult<T>(result: Result<T>) {
  if (result.success) {
    console.log(result.data);  // T
  } else {
    console.error(result.error);  // Error
  }
}
```

## Assertion Functions

```typescript
function assertDefined<T>(value: T | null | undefined): asserts value is T {
  if (value === null || value === undefined) {
    throw new Error('Value must be defined');
  }
}

function processUser(user: User | null) {
  assertDefined(user);
  // user is now User, not User | null
  console.log(user.name);
}
```

---

# 🎯 Advanced Patterns

## Mapped Types

```typescript
// Make all properties nullable
type Nullable<T> = {
  [K in keyof T]: T[K] | null;
};

// Make all properties promises
type Async<T> = {
  [K in keyof T]: Promise<T[K]>;
};

// Prefix all keys
type Prefixed<T, P extends string> = {
  [K in keyof T as `${P}${string & K}`]: T[K];
};

type PrefixedUser = Prefixed<User, 'user_'>;
// { user_id: string; user_name: string; user_email: string }
```

## Conditional Types

```typescript
// Extract array element type
type ArrayElement<T> = T extends (infer E)[] ? E : never;

type StringArray = string[];
type Element = ArrayElement<StringArray>;  // string

// Exclude null/undefined
type NonNullable<T> = T extends null | undefined ? never : T;

// Function return type
type ReturnType<T> = T extends (...args: any[]) => infer R ? R : never;
```

## Template Literal Types

```typescript
type EventName = 'click' | 'focus' | 'blur';
type EventHandler = `on${Capitalize<EventName>}`;
// 'onClick' | 'onFocus' | 'onBlur'

type HttpMethod = 'get' | 'post' | 'put' | 'delete';
type Endpoint = `/${string}`;
type Route = `${Uppercase<HttpMethod>} ${Endpoint}`;
// 'GET /...' | 'POST /...' | ...
```

---

# 📦 React TypeScript Patterns

## Component Props

```typescript
interface ButtonProps {
  variant?: 'primary' | 'secondary' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  disabled?: boolean;
  onClick?: () => void;
  children: React.ReactNode;
}

// With HTML attributes
interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary';
  loading?: boolean;
}

export function Button({ 
  variant = 'primary',
  loading,
  children,
  ...props 
}: ButtonProps) {
  return (
    <button disabled={loading} {...props}>
      {loading ? <Spinner /> : children}
    </button>
  );
}
```

## Generic Components

```typescript
interface ListProps<T> {
  items: T[];
  renderItem: (item: T) => React.ReactNode;
  keyExtractor: (item: T) => string;
}

export function List<T>({ items, renderItem, keyExtractor }: ListProps<T>) {
  return (
    <ul>
      {items.map(item => (
        <li key={keyExtractor(item)}>{renderItem(item)}</li>
      ))}
    </ul>
  );
}

// Usage
<List
  items={users}
  renderItem={user => <span>{user.name}</span>}
  keyExtractor={user => user.id}
/>
```

## Hooks Types

```typescript
// useState with complex type
const [user, setUser] = useState<User | null>(null);

// useReducer
type Action = 
  | { type: 'SET_USER'; payload: User }
  | { type: 'LOGOUT' };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'SET_USER':
      return { ...state, user: action.payload };
    case 'LOGOUT':
      return { ...state, user: null };
  }
}

// Custom hook return type
function useUser(): {
  user: User | null;
  loading: boolean;
  login: (credentials: Credentials) => Promise<void>;
  logout: () => void;
} {
  // ...
}
```

---

# 🔒 Strict Configuration

```json
// tsconfig.json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true
  }
}
```

---

# 📚 References

- [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/)
- [Type Challenges](https://github.com/type-challenges/type-challenges)
- [Matt Pocock's Total TypeScript](https://www.totaltypescript.com/)
