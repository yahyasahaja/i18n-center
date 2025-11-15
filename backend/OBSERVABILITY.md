# Observability & Monitoring

This document describes the comprehensive observability implementation for the i18n-center service using Datadog.

## Overview

The service implements a complete observability stack with:
- **Structured Logging** (zap) - **Always enabled**
- **Metrics** (Datadog StatsD) - **Optional** (set `DD_ENABLED=true` to enable)
- **Distributed Tracing** (Datadog APM) - **Optional** (set `DD_ENABLED=true` to enable)
- **Error Tracking** (with context) - **Always enabled**
- **Panic Recovery** (graceful error handling) - **Always enabled**
- **Health Checks** (for Kubernetes/Docker) - **Always enabled**

### Local Development

**The service works perfectly without Datadog!** Just don't set `DD_ENABLED` or set it to `false`. All logging and error handling will work normally, and you won't need a Datadog agent running.

## Cost Optimization Strategies

To minimize Datadog costs while maintaining comprehensive observability:

### 1. **Sampling Rates**
- **Traces**: 5-10% sampling (100% for errors)
- **Metrics**: 100% for errors, 10% for successful requests
- **Cache Operations**: 10% sampling
- **Database Operations**: 100% (critical for debugging)

### 2. **Log Levels**
- **Production**: INFO and above only
- **Development**: DEBUG enabled
- Errors always logged with full context

### 3. **Metric Cardinality**
- Path normalization to prevent UUID explosion
- Status code grouping (2xx, 4xx, 5xx)
- Limited tag values per metric

### 4. **Trace Sampling**
- Automatic error sampling (100%)
- Configurable rate for normal requests
- Environment-based configuration

## Components

### 1. Structured Logging (`observability/logger.go`)

**Features:**
- JSON-formatted logs (production) or console (development)
- Automatic service metadata (service name, environment, version)
- Stack traces for errors
- Contextual logging with fields

**Usage:**
```go
observability.Logger.Info("Operation completed", zap.String("user_id", userID))
observability.LogError(err, "Failed to process", zap.String("operation", "save"))
```

### 2. Metrics (`observability/metrics.go`)

**Metrics Tracked:**
- `http.requests` - Request count by method, path, status
- `http.request.duration` - Request latency
- `http.errors.server` - Server errors (5xx)
- `http.errors.client` - Client errors (4xx)
- `db.operations` - Database operation count
- `db.duration` - Database operation latency
- `db.errors` - Database errors
- `cache.operations` - Cache operation count
- `cache.duration` - Cache operation latency
- `service.health` - Service health status (0 or 1)
- `service.panics` - Panic occurrences

**Tags:**
- `method` - HTTP method
- `path` - Request path (normalized)
- `status` - HTTP status code
- `status_class` - Status class (2xx, 4xx, 5xx)
- `operation` - Operation type (query, create, update, delete)
- `error` - Error flag (true/false)
- `hit` - Cache hit/miss

### 3. Distributed Tracing (`observability/tracing.go`)

**Features:**
- Automatic HTTP request tracing
- Database query tracing (via GORM callbacks)
- Cache operation tracing
- Error tagging in traces
- Sampling for cost efficiency

**Trace Tags:**
- `http.method` - HTTP method
- `http.url` - Request path
- `http.status_code` - Response status
- `http.latency_ms` - Request latency
- `error` - Error flag
- `error.type` - Error type
- `error.message` - Error message

### 4. Middleware

#### ObservabilityMiddleware
- Tracks request latency
- Logs all HTTP requests
- Records metrics
- Creates distributed traces
- Tags errors in traces

#### PanicRecoveryMiddleware
- Recovers from panics gracefully
- Logs panic with full context
- Records panic metrics
- Returns 500 error to client

#### ErrorLoggingMiddleware
- Captures Gin errors
- Logs with context (method, path, status, user)
- Tracks error types

### 5. Health Checks (`handlers/health_handler.go`)

**Endpoints:**
- `GET /health` - Full health check (database, dependencies)
- `GET /ready` - Readiness probe (for Kubernetes)
- `GET /live` - Liveness probe (for Kubernetes)

**Response:**
```json
{
  "status": "healthy",
  "timestamp": 1234567890,
  "service": "i18n-center",
  "version": "1.0.0",
  "database": {
    "status": "healthy"
  }
}
```

## Environment Variables

Add these to your `.env` file:

```env
# Observability
ENV=production                    # or development
VERSION=1.0.0                     # Service version

# Datadog (optional - service works without it)
DD_ENABLED=true                   # Set to false or leave empty to disable Datadog (default: disabled)
DD_AGENT_HOST=localhost           # Datadog agent host (or datadog-agent service in K8s)
DD_DOGSTATSD_PORT=8125            # StatsD port (default: 8125)
DD_SERVICE=i18n-center            # Service name (optional, defaults to i18n-center)
DD_ENV=production                 # Environment (optional, uses ENV)
DD_VERSION=1.0.0                  # Version (optional, uses VERSION)
```

### Local Development (Without Datadog)

For local development, you can simply **not set `DD_ENABLED`** or set it to `false`:

```env
# Local development - no Datadog needed
ENV=development
VERSION=1.0.0
# DD_ENABLED=false  # or just don't set it
```

The service will:
- ✅ Work completely normally
- ✅ Still log everything (structured logging is always enabled)
- ✅ Skip metrics collection (no errors)
- ✅ Skip tracing (no errors)
- ✅ All health checks work
- ✅ No Datadog agent required

## Datadog Agent Setup

### Local Development
```bash
# Install Datadog agent (macOS)
brew install datadog-agent

# Or use Docker
docker run -d --name datadog-agent \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /proc/:/host/proc/:ro \
  -v /sys/fs/cgroup/:/host/sys/fs/cgroup:ro \
  -e DD_API_KEY=your-api-key \
  -e DD_SITE=datadoghq.com \
  -p 8125:8125/udp \
  datadog/agent:latest
```

### Kubernetes
The Datadog agent should be deployed as a DaemonSet. The service will automatically connect to it via the `datadog-agent` service.

## Monitoring Dashboards

### Recommended Datadog Dashboards

1. **Service Overview**
   - Request rate
   - Error rate
   - P95/P99 latency
   - Service health

2. **Database Performance**
   - Query count by operation
   - Query latency (P50, P95, P99)
   - Error rate
   - Slow queries

3. **Cache Performance**
   - Cache hit rate
   - Cache operation latency
   - Cache errors

4. **Error Tracking**
   - Error rate by endpoint
   - Error types
   - Panic occurrences
   - Error trends

## Alerts

### Recommended Alerts

1. **High Error Rate**
   - Alert when error rate > 5% for 5 minutes
   - Metric: `http.errors.server` + `http.errors.client`

2. **High Latency**
   - Alert when P95 latency > 1s for 5 minutes
   - Metric: `http.request.duration`

3. **Service Down**
   - Alert when `service.health` = 0
   - Metric: `service.health`

4. **Database Issues**
   - Alert when database error rate > 1%
   - Metric: `db.errors`

5. **Panic Detection**
   - Alert on any panic
   - Metric: `service.panics`

## Cost Estimation

### Typical Monthly Costs (for reference)

**Low Traffic (< 1M requests/month):**
- Logs: ~$50-100/month
- Metrics: ~$30-50/month
- Traces: ~$20-40/month
- **Total: ~$100-190/month**

**Medium Traffic (1-10M requests/month):**
- Logs: ~$200-400/month
- Metrics: ~$100-200/month
- Traces: ~$50-100/month
- **Total: ~$350-700/month**

**Optimization Tips:**
1. Use sampling rates (already implemented)
2. Filter verbose logs
3. Aggregate similar log entries
4. Use appropriate log levels
5. Monitor metric cardinality
6. Set log retention periods

## Troubleshooting

### Metrics Not Appearing
1. Check Datadog agent is running: `docker ps | grep datadog`
2. Verify `DD_AGENT_HOST` and `DD_DOGSTATSD_PORT` are correct
3. Check agent logs: `docker logs datadog-agent`
4. Verify network connectivity

### Traces Not Appearing
1. Check APM is enabled in Datadog agent
2. Verify sampling rate is not too low
3. Check trace ingestion limits in Datadog

### High Costs
1. Review sampling rates
2. Check log volume and levels
3. Audit metric cardinality
4. Review trace sampling rates
5. Check for duplicate metrics/logs

## Best Practices

1. **Always log errors with context**
   ```go
   observability.LogError(err, "Operation failed",
     zap.String("user_id", userID),
     zap.String("operation", "save"))
   ```

2. **Use appropriate log levels**
   - ERROR: Errors that need attention
   - WARN: Warnings that might indicate issues
   - INFO: Important business events
   - DEBUG: Detailed debugging information

3. **Tag metrics consistently**
   - Use standard tag names
   - Limit tag value cardinality
   - Group related metrics

4. **Monitor costs regularly**
   - Set up cost alerts
   - Review usage weekly
   - Optimize based on actual usage

5. **Test observability in staging**
   - Verify all metrics are being sent
   - Check trace sampling
   - Validate log formats

