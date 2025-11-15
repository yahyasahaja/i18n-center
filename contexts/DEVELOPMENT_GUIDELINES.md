# Development Guidelines

## Code Style

### Go (Backend)

**Formatting:**
- Use `gofmt` or `goimports`
- Follow standard Go conventions
- Use `golangci-lint` for linting

**Naming:**
- Exported: PascalCase
- Unexported: camelCase
- Constants: PascalCase or UPPER_CASE
- Interfaces: `-er` suffix (e.g., `Reader`, `Writer`)

**File Organization:**
```go
package handlers

import (
    // Standard library
    "fmt"
    "net/http"

    // Third-party
    "github.com/gin-gonic/gin"

    // Local
    "github.com/your-org/i18n-center/models"
)
```

**Error Handling:**
```go
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}
```

**Comments:**
- Export all public functions/types
- Use godoc format
- Explain "why", not "what"

### TypeScript (Frontend)

**Formatting:**
- Use Prettier
- ESLint for linting
- 2-space indentation

**Naming:**
- Components: PascalCase
- Functions: camelCase
- Constants: UPPER_CASE
- Types/Interfaces: PascalCase

**File Organization:**
```typescript
// Imports
import React from 'react'
import { useDispatch } from 'react-redux'

// Types
interface Props {
  // ...
}

// Component
export const MyComponent: React.FC<Props> = ({ ... }) => {
  // ...
}
```

## Git Workflow

### Branch Strategy

- `main`: Production-ready code
- `develop`: Development branch
- `feature/*`: Feature branches
- `fix/*`: Bug fixes
- `hotfix/*`: Production hotfixes

### Commit Messages

**Format:**
```
type(scope): subject

body (optional)

footer (optional)
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code refactoring
- `test`: Tests
- `chore`: Maintenance

**Examples:**
```
feat(translations): add version comparison endpoint
fix(auth): handle expired token gracefully
docs(api): update authentication section
```

### Pull Requests

1. Create feature branch
2. Make changes
3. Write tests
4. Update documentation
5. Create PR with description
6. Request review
7. Address feedback
8. Merge after approval

## Testing

### Backend Testing

**Location:** `backend/*_test.go`

**Coverage Target:** 80%

**Example:**
```go
func TestLogin(t *testing.T) {
    // Setup
    // Execute
    // Assert
}
```

**Run Tests:**
```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Frontend Testing

Currently not required, but can add:
- Jest for unit tests
- React Testing Library for components
- Cypress for E2E tests

## Code Review Checklist

### Backend
- [ ] Code compiles without errors
- [ ] Tests pass
- [ ] Error handling is proper
- [ ] No hardcoded values
- [ ] Database queries are safe (no SQL injection)
- [ ] Logging is appropriate
- [ ] Documentation is updated
- [ ] Follows Go conventions

### Frontend
- [ ] TypeScript compiles without errors
- [ ] No console errors
- [ ] Responsive design works
- [ ] Accessibility considerations
- [ ] Error handling is proper
- [ ] Loading states are shown
- [ ] No hardcoded values
- [ ] Documentation is updated

## Adding New Features

### Backend

1. **Create Model** (if needed)
   - Add to `models/models.go`
   - Add GORM tags
   - Add indexes if needed

2. **Create Service**
   - Add to `services/`
   - Implement business logic
   - Handle errors

3. **Create Handler**
   - Add to `handlers/`
   - Add Swagger annotations
   - Handle request/response
   - Call service

4. **Add Routes**
   - Add to `routes/routes.go`
   - Add middleware (auth, roles)
   - Test endpoint

5. **Write Tests**
   - Unit tests for service
   - Integration tests for handler

6. **Update Documentation**
   - Update API_DOCUMENTATION.md
   - Update contexts/ files

### Frontend

1. **Create/Update Types**
   - Add to Redux slice or types file

2. **Create Component**
   - Add to `components/`
   - Use existing UI components
   - Handle loading/error states

3. **Add API Method**
   - Add to `services/api.ts`

4. **Add Redux Slice** (if needed)
   - Add to `store/slices/`
   - Add actions/thunks

5. **Create Page**
   - Add to `app/`
   - Use component
   - Handle routing

6. **Test**
   - Test all flows
   - Test error cases
   - Test edge cases

## Common Patterns

### Backend Handler Pattern

```go
func (h *Handler) CreateResource(c *gin.Context) {
    // 1. Parse request
    var req Request
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 2. Validate
    // ...

    // 3. Call service
    result, err := h.service.Create(req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 4. Return response
    c.JSON(http.StatusCreated, result)
}
```

### Frontend API Call Pattern

```typescript
const handleSave = async () => {
  setLoading(true)
  try {
    const response = await api.save(data)
    toast.success('Saved successfully')
    // Update state
  } catch (error: any) {
    toast.error(error.response?.data?.error || 'Failed to save')
  } finally {
    setLoading(false)
  }
}
```

## Performance Guidelines

### Backend
- Use database indexes
- Cache frequently queried data
- Use connection pooling
- Avoid N+1 queries
- Paginate large results

### Frontend
- Lazy load components
- Memoize expensive computations
- Optimize re-renders
- Code split routes
- Optimize bundle size

## Security Guidelines

### Backend
- Never log sensitive data
- Validate all inputs
- Use parameterized queries
- Hash passwords (bcrypt)
- Use HTTPS in production
- Validate JWT tokens
- Check permissions

### Frontend
- Never store tokens in localStorage (consider httpOnly cookies)
- Sanitize user inputs
- Validate on client and server
- Use HTTPS
- Handle errors gracefully (don't expose internals)

## Documentation

### Code Comments
- Document public APIs
- Explain complex logic
- Add TODO comments for future work
- Use godoc format (Go)

### README Files
- Keep README.md updated
- Document setup steps
- Include examples
- List dependencies

### Context Documentation
- Update contexts/ when making changes
- Document architectural decisions
- Explain "why", not just "what"

## Debugging

### Backend
- Use structured logging
- Add request IDs for tracing
- Use debugger (Delve)
- Check database queries
- Monitor Redis cache

### Frontend
- Use React DevTools
- Use Redux DevTools
- Check browser console
- Use Network tab
- Check Redux state

## Dependencies

### Adding Dependencies

**Backend:**
```bash
go get package-name
go mod tidy
```

**Frontend:**
```bash
yarn add package-name
```

### Updating Dependencies

**Backend:**
```bash
go get -u ./...
go mod tidy
```

**Frontend:**
```bash
yarn upgrade
```

### Security Updates

- Regularly update dependencies
- Check for vulnerabilities
- Use `go list -m -u all` (Go)
- Use `yarn audit` (Frontend)

## Code Review Process

1. **Self-Review**
   - Check your own code
   - Run tests
   - Check linting

2. **Create PR**
   - Clear description
   - Link to issue
   - Screenshots if UI change

3. **Review Feedback**
   - Address all comments
   - Ask for clarification
   - Update code

4. **Merge**
   - Squash commits
   - Clean commit history
   - Delete branch after merge

## Best Practices

1. **Keep it Simple**
   - Don't over-engineer
   - YAGNI principle
   - KISS principle

2. **DRY (Don't Repeat Yourself)**
   - Extract common code
   - Reuse components
   - Share utilities

3. **SOLID Principles**
   - Single Responsibility
   - Open/Closed
   - Liskov Substitution
   - Interface Segregation
   - Dependency Inversion

4. **Error Handling**
   - Always handle errors
   - Provide meaningful messages
   - Log appropriately

5. **Testing**
   - Write tests first (TDD if possible)
   - Test edge cases
   - Maintain coverage

