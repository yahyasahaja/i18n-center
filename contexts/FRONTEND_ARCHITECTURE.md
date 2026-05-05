# Frontend Architecture

## Overview

Next.js 14+ application using App Router, TypeScript, TailwindCSS, and Redux Toolkit.

## Tech Stack

- **Framework:** Next.js 14+ (App Router)
- **Language:** TypeScript
- **Styling:** TailwindCSS
- **State Management:** Redux Toolkit
- **Code Editor:** Monaco Editor
- **Diff Viewer:** react-diff-viewer-continued
- **HTTP Client:** Axios (via custom API service)

## Project Structure

```
frontend/
├── app/                      # Next.js App Router
│   ├── (auth)/               # Auth group
│   │   ├── login/
│   │   └── layout.tsx
│   ├── applications/         # Application pages
│   │   ├── page.tsx         # List
│   │   └── [id]/
│   │       └── page.tsx     # Detail
│   ├── components/           # Component pages
│   │   ├── page.tsx         # List
│   │   └── [id]/
│   │       └── translations/
│   │           └── page.tsx  # Translation editor
│   ├── dashboard/
│   │   └── page.tsx
│   ├── users/
│   │   └── page.tsx
│   ├── layout.tsx            # Root layout
│   ├── page.tsx              # Root redirect
│   └── providers.tsx         # Redux provider
├── components/              # React components
│   ├── ui/                   # Base UI components
│   │   ├── Button.tsx
│   │   ├── Input.tsx
│   │   ├── Modal.tsx
│   │   ├── Select.tsx
│   │   ├── Table.tsx
│   │   └── ...
│   ├── CodeEditor.tsx        # Monaco editor wrapper
│   ├── DiffView.tsx           # Diff viewer
│   ├── TranslationEditor.tsx # Main translation editor
│   └── Sidebar.tsx           # Navigation sidebar
├── store/                     # Redux store
│   ├── index.ts              # Store configuration
│   └── slices/               # Redux slices
│       ├── authSlice.ts
│       ├── applicationSlice.ts
│       ├── componentSlice.ts
│       └── ...
├── services/                  # API client
│   └── api.ts                # Axios instance + API methods
├── hooks/                     # Custom React hooks
│   └── useAuth.ts            # Auth hook
└── middleware.ts             # Next.js middleware
```

## State Management

### Redux Store Structure

```typescript
{
  auth: {
    token: string | null
    user: User | null
    isAuthenticated: boolean
  },
  applications: {
    items: Application[]
    current: Application | null
    loading: boolean
  },
  components: {
    items: Component[]
    current: Component | null
    loading: boolean
  }
}
```

### Redux Slices

**Location:** `frontend/store/slices/`

Each slice manages:
- State
- Actions (async thunks)
- Reducers

**Note on CMS pages:** The CMS pages (`/cms/templates`, `/cms/items`, `/cms/items/[id]`) use **local React state** (useState/useEffect) rather than Redux slices. This keeps CMS state self-contained within each page component. Only the auth state (token, user) is read from the Redux store in CMS pages.

**Example:**
```typescript
const applicationSlice = createSlice({
  name: 'applications',
  initialState: { items: [], current: null, loading: false },
  reducers: { ... },
  extraReducers: (builder) => {
    builder
      .addCase(fetchApplications.pending, (state) => {
        state.loading = true
      })
      .addCase(fetchApplications.fulfilled, (state, action) => {
        state.items = action.payload
        state.loading = false
      })
  }
})
```

## Routing

### App Router Structure

- `/` - Root redirect
- `/login` - Login page
- `/dashboard` - Dashboard
- `/applications` - Application list
- `/applications/[id]` - Application detail
- `/components` - Component list
- `/components/[id]/translations` - Translation editor
- `/users` - User management
- `/cms/templates` - CMS template management (list + CRUD modal)
- `/cms/items` - CMS item list + CRUD modal
- `/cms/items/[id]` - CMS item localization editor (locale tabs, stage tabs, field editors by type, translate-from buttons, deploy buttons, version history modal)

### Route Protection

**Implementation:** Client-side checks in page components

```typescript
useEffect(() => {
  const token = localStorage.getItem('token')
  if (!token) {
    router.replace('/login')
    return
  }
  // Load data...
}, [])
```

### Role-Based Access

```typescript
useEffect(() => {
  if (user && user.role !== 'super_admin' && user.role !== 'user_manager') {
    router.replace('/dashboard')
  }
}, [user, router])
```

## API Client

### Axios Instance

**Location:** `frontend/services/api.ts`

```typescript
const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor: Add token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor: Handle errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)
```

### API Methods

Organized by resource:

```typescript
export const applicationApi = {
  getAll: () => api.get('/applications'),
  getById: (id: string) => api.get(`/applications/${id}`),
  create: (data: ApplicationRequest) => api.post('/applications', data),
  update: (id: string, data: ApplicationRequest) =>
    api.put(`/applications/${id}`, data),
  delete: (id: string) => api.delete(`/applications/${id}`),
}
```

## UI Components

### Base Components

**Location:** `frontend/components/ui/`

- `Button`: Primary, secondary, outline variants
- `Input`: Text input with validation
- `Textarea`: Multi-line input
- `Select`: Dropdown select
- `Modal`: Dialog/modal component
- `Table`: Data table
- `Card`: Container card
- `Badge`: Status badge

### RichTextEditor

**Component:** `frontend/components/RichTextEditor.tsx`

TipTap-based WYSIWYG rich text editor used in the CMS item localization editor for fields with `value_type: rich_text`.

**Features:**
- Full WYSIWYG editing (bold, italic, headings, lists, links)
- Image upload via `POST /api/cms/upload-image` (GCS backend, optional)
- Returns/accepts HTML strings
- Renders inline within the CMS localization editor field list

### Styling

**TailwindCSS** with custom theme:

```typescript
// tailwind.config.js
theme: {
  extend: {
    colors: {
      primary: {
        50: '#...',
        600: '#...',
        // ...
      }
    }
  }
}
```

**Color Scheme:**
- Primary: Blue
- Success: Green
- Warning: Yellow
- Danger: Red
- Gray: Neutral

## Key Features

### Translation Editor

**Component:** `frontend/components/TranslationEditor.tsx`

**Features:**
- Monaco Editor for JSON editing
- Real-time validation
- Duplicate key detection
- Unsaved changes warning
- Version comparison
- Diff viewer
- Locale/stage selection
- Save/Revert/Deploy actions
- Auto-translate
- Backfill
- Export/Import

**State Management:**
- Local state for editor content
- Redux for component/application data
- Original JSON tracking for change detection

### Code Editor

**Component:** `frontend/components/CodeEditor.tsx`

**Monaco Editor Integration:**
- JSON syntax highlighting
- Auto-formatting
- Error detection
- Line numbers
- Word wrap

### Diff Viewer

**Component:** `frontend/components/DiffView.tsx`

**Features:**
- Side-by-side comparison
- Syntax highlighting
- Line-by-line diff
- Before/After labels

## Authentication Flow

### Login

1. User submits credentials
2. API call to `/api/auth/login`
3. Store token in `localStorage`
4. Store user in Redux
5. Redirect to dashboard

### Logout

1. Clear `localStorage`
2. Clear Redux state
3. Redirect to login

### Token Refresh

Currently not implemented. Token expires after 24 hours, user must re-login.

## Error Handling

### Toast Notifications

**Library:** `react-hot-toast`

**Usage:**
```typescript
import toast from 'react-hot-toast'

toast.success('Saved successfully')
toast.error('Failed to save')
```

### API Errors

Handled in:
1. Axios interceptors (401 → redirect to login)
2. Component error boundaries
3. Toast notifications

## Performance Optimizations

### Implemented
- Code splitting (Next.js automatic)
- Lazy loading of Monaco Editor
- Redux state normalization
- Memoization where needed

### Future
- [ ] React Query for caching
- [ ] Virtual scrolling for large lists
- [ ] Image optimization
- [ ] Bundle size optimization

## Environment Variables

```env
NEXT_PUBLIC_API_URL=http://localhost:8080/api
```

## Build & Deployment

### Development
```bash
yarn dev
```

### Production Build
```bash
yarn build
yarn start
```

### Docker
```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile
COPY . .
RUN yarn build
CMD ["yarn", "start"]
```

## Best Practices

1. **Type Safety:** Use TypeScript strictly
2. **Component Reusability:** Extract common UI components
3. **State Management:** Use Redux for global state, local state for UI
4. **Error Handling:** Always handle errors gracefully
5. **Loading States:** Show loading indicators
6. **Validation:** Validate inputs before submission
7. **Accessibility:** Use semantic HTML, ARIA labels
8. **Performance:** Optimize renders, use memoization

