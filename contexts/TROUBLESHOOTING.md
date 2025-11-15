# Troubleshooting Guide

## Common Issues & Solutions

### Backend Issues

#### Database Connection Failed

**Symptoms:**
- `Failed to initialize database`
- `connection refused`
- `authentication failed`

**Solutions:**
1. Check database is running:
   ```bash
   docker ps | grep postgres
   ```

2. Verify connection string:
   ```env
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=password
   DB_NAME=i18n_center
   ```

3. Check PostgreSQL logs:
   ```bash
   docker logs postgres-container
   ```

4. Test connection:
   ```bash
   psql -h localhost -U postgres -d i18n_center
   ```

#### Redis Connection Failed

**Symptoms:**
- `Warning: Failed to initialize Redis cache`
- Backend continues without cache

**Solutions:**
1. Check Redis is running:
   ```bash
   docker ps | grep redis
   ```

2. Verify connection:
   ```bash
   redis-cli ping
   ```

3. Backend gracefully degrades without Redis (cache disabled)

#### JWT Token Invalid

**Symptoms:**
- `401 Unauthorized`
- Token not accepted

**Solutions:**
1. Check JWT_SECRET is set:
   ```env
   JWT_SECRET=your-secret-key
   ```

2. Verify token format:
   ```
   Authorization: Bearer <token>
   ```

3. Check token expiration (24 hours)

4. Re-login to get new token

#### Port Already in Use

**Symptoms:**
- `bind: address already in use`
- Server won't start

**Solutions:**
1. Find process using port:
   ```bash
   lsof -i :8080
   ```

2. Kill process:
   ```bash
   kill -9 <PID>
   ```

3. Or change port:
   ```env
   PORT=8081
   ```

#### Migration Errors

**Symptoms:**
- `column already exists`
- `table already exists`
- `column "code" of relation "applications" contains null values (SQLSTATE 23502)`

**Solutions:**

1. **Code Field Migration Error:**
   - **Cause**: Trying to add NOT NULL column to table with existing rows
   - **Solution**: The `migrateCodeFields()` function handles this automatically:
     - Adds column as nullable first
     - Backfills existing rows with generated codes
     - Makes column NOT NULL after backfill
   - **Manual Fix** (if needed):
     ```sql
     -- Add column as nullable
     ALTER TABLE applications ADD COLUMN code text;

     -- Backfill codes
     UPDATE applications
     SET code = LOWER(REGEXP_REPLACE(name, '[^a-zA-Z0-9]+', '_', 'g'))
     WHERE code IS NULL;

     -- Make NOT NULL
     ALTER TABLE applications ALTER COLUMN code SET NOT NULL;
     ```

2. **General Migration Errors:**
   - Check if migration already ran
   - Manually fix schema if needed
   - Reset database (development only):
     ```bash
     docker-compose down -v
     docker-compose up -d
     ```

### Frontend Issues

#### API Connection Failed

**Symptoms:**
- `Network Error`
- `CORS error`
- `Failed to fetch`

**Solutions:**
1. Check backend is running:
   ```bash
   curl http://localhost:8080/api/auth/login
   ```

2. Verify API URL:
   ```env
   NEXT_PUBLIC_API_URL=http://localhost:8080/api
   ```

3. Check CORS settings in backend

4. Check browser console for errors

#### Login Redirect Loop

**Symptoms:**
- Redirects to login after successful login
- Can't stay logged in

**Solutions:**
1. Check token is stored:
   ```javascript
   localStorage.getItem('token')
   ```

2. Verify auth state initialization:
   - Check `AuthInitializer` in `providers.tsx`
   - Check `useAuth` hook

3. Check middleware not blocking:
   - Verify `middleware.ts` allows requests

4. Clear localStorage and try again:
   ```javascript
   localStorage.clear()
   ```

#### Monaco Editor Not Loading

**Symptoms:**
- Editor shows "Loading..." forever
- Editor doesn't appear

**Solutions:**
1. Check Monaco Editor is installed:
   ```bash
   yarn list @monaco-editor/react
   ```

2. Check browser console for errors

3. Verify editor component:
   ```typescript
   import Editor from '@monaco-editor/react'
   ```

4. Check network tab for Monaco assets

#### Redux State Not Updating

**Symptoms:**
- State changes but UI doesn't update
- Old data persists

**Solutions:**
1. Check Redux DevTools
2. Verify action is dispatched
3. Check reducer is handling action
4. Verify component is connected:
   ```typescript
   const items = useAppSelector((state) => state.items.items)
   ```

#### Build Errors

**Symptoms:**
- `Type error: Property does not exist`
- Build fails

**Solutions:**
1. Check TypeScript types match backend
2. Run type check:
   ```bash
   yarn tsc --noEmit
   ```

3. Check for missing imports
4. Verify types are exported

### Database Issues

#### Duplicate Key Error

**Symptoms:**
- `duplicate key value violates unique constraint`
- Can't create resource

**Solutions:**
1. Check unique constraint:
   - Application name must be unique
   - Username must be unique

2. Use different value
3. Check if resource already exists

#### Foreign Key Constraint

**Symptoms:**
- `foreign key constraint fails`
- Can't delete resource

**Solutions:**
1. Check dependent records:
   - Can't delete Application with Components
   - Can't delete Component with Translations

2. Delete dependent records first
3. Or use cascade delete (if configured)

#### JSONB Parsing Error

**Symptoms:**
- `invalid input syntax for type jsonb`
- Can't save translation

**Solutions:**
1. Verify JSON is valid:
   ```go
   json.Valid([]byte(jsonString))
   ```

2. Check for duplicate keys
3. Validate JSON structure

### Deployment Issues

#### Image Build Fails

**Symptoms:**
- Docker build fails
- Go build errors

**Solutions:**
1. Check Go version matches `go.mod`
2. Verify dependencies:
   ```bash
   go mod download
   ```

3. Check Dockerfile syntax
4. Build locally first:
   ```bash
   docker build -t test .
   ```

#### Kubernetes Pod Not Starting

**Symptoms:**
- Pod in CrashLoopBackOff
- Pod not ready

**Solutions:**
1. Check pod logs:
   ```bash
   kubectl logs <pod-name>
   ```

2. Check pod events:
   ```bash
   kubectl describe pod <pod-name>
   ```

3. Verify environment variables
4. Check resource limits

#### CloudSQL Connection Issues

**Symptoms:**
- Can't connect to CloudSQL
- Connection timeout

**Solutions:**
1. Check CloudSQL Proxy is running
2. Verify instance name in proxy command
3. Check network connectivity
4. Verify credentials

### Performance Issues

#### Slow API Responses

**Symptoms:**
- API takes long to respond
- Timeout errors

**Solutions:**
1. Check database queries:
   - Use indexes
   - Avoid N+1 queries
   - Use pagination

2. Check Redis cache:
   - Verify cache is working
   - Check cache hit rate

3. Check database performance:
   - Slow queries
   - Missing indexes

#### High Memory Usage

**Symptoms:**
- Out of memory errors
- Pod killed

**Solutions:**
1. Check for memory leaks
2. Review cache TTL
3. Check Redis memory usage
4. Increase resource limits

### Debugging Tips

#### Backend Debugging

1. **Enable Debug Logging:**
   ```go
   gin.SetMode(gin.DebugMode)
   ```

2. **Add Request Logging:**
   ```go
   r.Use(gin.Logger())
   ```

3. **Use Debugger:**
   ```bash
   dlv debug
   ```

4. **Check Database Queries:**
   ```go
   database.DB = database.DB.Debug()
   ```

#### Frontend Debugging

1. **React DevTools:**
   - Install browser extension
   - Inspect component tree
   - Check props/state

2. **Redux DevTools:**
   - Install browser extension
   - Inspect Redux state
   - Time travel debugging

3. **Network Tab:**
   - Check API requests
   - Verify responses
   - Check headers

4. **Console Logging:**
   ```typescript
   console.log('Debug:', data)
   ```

## Getting Help

1. Check this troubleshooting guide
2. Check contexts/ documentation
3. Review error logs
4. Check GitHub issues
5. Ask team for help

## Log Locations

### Backend
- Console output (development)
- Cloud Logging (production)
- Application logs

### Frontend
- Browser console
- Network tab
- Redux DevTools

### Database
- PostgreSQL logs (Docker or CloudSQL)
- Query logs (if enabled)

### Redis
- Redis logs (Docker or managed)
- Connection logs

