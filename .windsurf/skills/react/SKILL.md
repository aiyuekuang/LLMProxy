---
name: react
description: React best practices and patterns. Use for component design, hooks, state management, and React performance optimization.
---

# React Skill

React patterns, hooks, and best practices from Vercel and industry standards.

## When to Use This Skill

- Building React components
- Managing state with hooks
- React Query / data fetching
- Performance optimization
- Component composition

---

# 📐 Component Patterns

## Functional Components

```tsx
interface UserCardProps {
  user: User;
  onEdit?: (id: string) => void;
}

export function UserCard({ user, onEdit }: UserCardProps) {
  return (
    <div className="rounded-lg border p-4">
      <h3 className="font-semibold">{user.name}</h3>
      <p className="text-gray-600">{user.email}</p>
      {onEdit && (
        <button onClick={() => onEdit(user.id)}>Edit</button>
      )}
    </div>
  );
}
```

## Compound Components

```tsx
const Tabs = ({ children, defaultValue }) => {
  const [active, setActive] = useState(defaultValue);
  return (
    <TabsContext.Provider value={{ active, setActive }}>
      {children}
    </TabsContext.Provider>
  );
};

Tabs.List = ({ children }) => (
  <div role="tablist" className="flex gap-2">{children}</div>
);

Tabs.Trigger = ({ value, children }) => {
  const { active, setActive } = useTabsContext();
  return (
    <button
      role="tab"
      aria-selected={active === value}
      onClick={() => setActive(value)}
    >
      {children}
    </button>
  );
};

// Usage
<Tabs defaultValue="tab1">
  <Tabs.List>
    <Tabs.Trigger value="tab1">Tab 1</Tabs.Trigger>
    <Tabs.Trigger value="tab2">Tab 2</Tabs.Trigger>
  </Tabs.List>
  <Tabs.Content value="tab1">Content 1</Tabs.Content>
</Tabs>
```

---

# 🪝 Hooks Best Practices

## useState

```tsx
// Simple state
const [count, setCount] = useState(0);

// Lazy initialization (for expensive computations)
const [data, setData] = useState(() => expensiveComputation());

// Functional updates
setCount(prev => prev + 1);
```

## useEffect

```tsx
// Fetch data
useEffect(() => {
  const controller = new AbortController();
  
  async function fetchData() {
    const res = await fetch('/api/data', { signal: controller.signal });
    setData(await res.json());
  }
  
  fetchData();
  
  return () => controller.abort();  // Cleanup
}, []);

// Event listener with cleanup
useEffect(() => {
  const handleResize = () => setWidth(window.innerWidth);
  window.addEventListener('resize', handleResize);
  return () => window.removeEventListener('resize', handleResize);
}, []);
```

## useMemo & useCallback

```tsx
// Memoize expensive calculations
const sortedItems = useMemo(() => {
  return items.sort((a, b) => a.name.localeCompare(b.name));
}, [items]);

// Memoize callbacks passed to children
const handleClick = useCallback((id: string) => {
  setSelected(id);
}, []);
```

## Custom Hooks

```tsx
function useLocalStorage<T>(key: string, initialValue: T) {
  const [value, setValue] = useState<T>(() => {
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : initialValue;
  });

  useEffect(() => {
    localStorage.setItem(key, JSON.stringify(value));
  }, [key, value]);

  return [value, setValue] as const;
}

// Usage
const [theme, setTheme] = useLocalStorage('theme', 'light');
```

---

# 📡 Data Fetching (React Query)

```tsx
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';

// Fetch data
function useUsers() {
  return useQuery({
    queryKey: ['users'],
    queryFn: () => fetch('/api/users').then(res => res.json()),
    staleTime: 5 * 60 * 1000,  // 5 minutes
  });
}

// Mutation with optimistic update
function useUpdateUser() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (user: User) => 
      fetch(`/api/users/${user.id}`, {
        method: 'PUT',
        body: JSON.stringify(user),
      }),
    onMutate: async (newUser) => {
      await queryClient.cancelQueries(['users', newUser.id]);
      const previous = queryClient.getQueryData(['users', newUser.id]);
      queryClient.setQueryData(['users', newUser.id], newUser);
      return { previous };
    },
    onError: (err, newUser, context) => {
      queryClient.setQueryData(['users', newUser.id], context?.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries(['users']);
    },
  });
}
```

---

# ⚡ Performance

## React.memo

```tsx
const ExpensiveList = memo(function ExpensiveList({ items }: { items: Item[] }) {
  return (
    <ul>
      {items.map(item => <li key={item.id}>{item.name}</li>)}
    </ul>
  );
});
```

## Virtualization

```tsx
import { FixedSizeList } from 'react-window';

function VirtualList({ items }) {
  return (
    <FixedSizeList
      height={400}
      itemCount={items.length}
      itemSize={50}
      width="100%"
    >
      {({ index, style }) => (
        <div style={style}>{items[index].name}</div>
      )}
    </FixedSizeList>
  );
}
```

## Code Splitting

```tsx
const Dashboard = lazy(() => import('./pages/Dashboard'));

function App() {
  return (
    <Suspense fallback={<LoadingSpinner />}>
      <Routes>
        <Route path="/dashboard" element={<Dashboard />} />
      </Routes>
    </Suspense>
  );
}
```

---

# 📚 References

- [React Documentation](https://react.dev/)
- [vercel-labs/react-best-practices](https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices)
- [callstackincubator/react-native-best-practices](https://github.com/callstackincubator/agent-skills)
