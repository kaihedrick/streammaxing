# SOP: Security Best Practices

## Overview
This document outlines the standard security practices that must be followed when developing, deploying, and maintaining StreamMaxing v3. These practices ensure the system remains secure against common vulnerabilities and threats.

---

## Core Security Principles

### 1. Defense in Depth
Implement multiple layers of security controls so that if one layer fails, others remain effective.

### 2. Principle of Least Privilege
Grant minimum necessary permissions to users, services, and components.

### 3. Fail Securely
Ensure that failures result in a secure state (deny access, log event, alert admins).

### 4. Never Trust User Input
Validate, sanitize, and escape all user input before processing or storing.

---

## Authentication & Authorization

### Session Management

**DO**:
- Use HTTP-only cookies for session tokens
- Set `Secure` flag in production (HTTPS only)
- Set `SameSite=Strict` for CSRF protection
- Use short JWT expiration times (24 hours max)
- Implement session revocation capability
- Generate unique JWT IDs (jti) for each session

**DON'T**:
- Store tokens in localStorage or sessionStorage
- Use long expiration times (>24 hours)
- Include sensitive data in JWT payload
- Allow token reuse after logout
- Skip CSRF protection

**Example**:
```go
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    jwtToken,
    Path:     "/",
    MaxAge:   86400,  // 24 hours
    HttpOnly: true,
    Secure:   isProd,
    SameSite: http.SameSiteStrictMode,
})
```

---

### Authorization Checks

**DO**:
- Re-validate permissions on EVERY request
- Use short-lived permission caches (5 minutes max)
- Check guild membership via Discord API
- Log all permission denial events
- Implement middleware for authorization checks

**DON'T**:
- Rely solely on cached permissions
- Trust client-side permission checks
- Skip authorization for "internal" endpoints
- Use database-only permission checks without API validation

**Example**:
```go
func (h *Handler) UpdateGuildConfig(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(string)
    guildID := chi.URLParam(r, "guild_id")

    // Always re-validate guild admin permission
    isAdmin, err := h.authService.CheckGuildAdmin(r.Context(), userID, guildID)
    if err != nil || !isAdmin {
        h.securityLogger.LogPermissionDenied(r.Context(), userID, guildID, "update_config")
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Continue with update...
}
```

---

## Data Protection

### Encryption at Rest

**DO**:
- Encrypt all OAuth tokens using AWS KMS
- Use strong encryption algorithms (AES-256)
- Rotate encryption keys annually
- Store encrypted data as base64 strings
- Keep plaintext data in memory only when needed

**DON'T**:
- Store OAuth tokens in plaintext
- Use custom encryption algorithms
- Hard-code encryption keys
- Log decrypted sensitive data

**Example**:
```go
// Encrypt before storing
encryptedToken, err := kmsService.Encrypt(accessToken)
if err != nil {
    securityLogger.LogTokenEncryptionFailure(ctx, streamerID, err)
    return err
}

// Store encrypted token
db.StoreToken(streamerID, encryptedToken)

// Decrypt when needed
plainToken, err := kmsService.Decrypt(encryptedToken)
// Use plainToken, then discard from memory
```

---

### Secrets Management

**DO**:
- Use AWS Secrets Manager for all secrets
- Rotate secrets every 90 days
- Cache secrets with 5-minute TTL
- Never commit secrets to git
- Use IAM roles for secret access

**DON'T**:
- Store secrets in environment variables (production)
- Hard-code secrets in source code
- Share secrets via email or chat
- Use the same secret across environments

**Example**:
```go
// Good: Load from Secrets Manager
secretsManager := secrets.NewSecretsManager()
jwtSecret, err := secretsManager.GetJWTSecret()

// Bad: Environment variable
jwtSecret := os.Getenv("JWT_SECRET") // Only for development
```

---

## Input Validation & Sanitization

### API Endpoints

**DO**:
- Validate all inputs before processing
- Check data types, formats, and ranges
- Limit payload sizes (1MB max for JSON)
- Sanitize inputs by removing dangerous characters
- Use allow-lists instead of deny-lists
- Return generic error messages to users

**DON'T**:
- Trust any user input
- Skip validation on "internal" endpoints
- Return detailed error messages that leak implementation details
- Process oversized payloads

**Example**:
```go
func (h *Handler) CreateResource(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Name  string `json:"name"`
        Value string `json:"value"`
    }

    // Limit request size
    r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB max

    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Validate
    if len(input.Name) == 0 || len(input.Name) > 100 {
        http.Error(w, "Name must be 1-100 characters", http.StatusBadRequest)
        return
    }

    // Sanitize
    input.Name = validator.SanitizeInput(input.Name)

    // Process...
}
```

---

### Template Content

**DO**:
- Validate template content for XSS attempts
- Limit template size (4KB max)
- Use allow-list for template variables
- Escape user-provided content in Discord messages
- Log suspicious template content

**DON'T**:
- Allow arbitrary HTML/JavaScript in templates
- Skip validation for "trusted" users
- Render templates without escaping

**Example**:
```go
func (v *Validator) ValidateTemplateContent(content string) error {
    // Check length
    if len(content) > 4000 {
        return fmt.Errorf("template too long")
    }

    // Check for dangerous patterns
    dangerous := []string{"<script", "javascript:", "onerror=", "onclick="}
    contentLower := strings.ToLower(content)
    for _, pattern := range dangerous {
        if strings.Contains(contentLower, pattern) {
            return fmt.Errorf("potentially dangerous content detected")
        }
    }

    return nil
}
```

---

## Rate Limiting & DDoS Protection

### API Rate Limiting

**DO**:
- Implement per-user rate limiting (50 req/min)
- Implement global rate limiting (1000 req/sec)
- Return 429 status code with `Retry-After` header
- Log rate limit violations
- Use token bucket algorithm for smoothing
- Cache limiters in memory with cleanup

**DON'T**:
- Skip rate limiting on any endpoint
- Use database for rate limiting (too slow)
- Allow unlimited requests for "admin" users
- Forget to clean up old rate limiter entries

**Example**:
```go
// Per-user rate limiter
rateLimiter := NewRateLimiter(50, 100) // 50 req/s, burst 100

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userID := r.Context().Value("user_id").(string)
        limiter := rl.getLimiter(userID)

        if !limiter.Allow() {
            w.Header().Set("Retry-After", "60")
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

---

### Webhook Protection

**DO**:
- Implement webhook-specific rate limiting (100 req/sec)
- Track message IDs for idempotency (15 min window)
- Verify HMAC signatures on ALL webhooks
- Check timestamp to prevent replay attacks (10 min window)
- Return 200 OK for duplicate messages
- Log signature verification failures

**DON'T**:
- Skip signature verification
- Process webhooks without idempotency check
- Allow old timestamps (>10 minutes)
- Retry webhook processing on failure

**Example**:
```go
func (h *WebhookHandler) HandleTwitchWebhook(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)

    // Verify signature
    messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
    timestamp := r.Header.Get("Twitch-Eventsub-Message-Timestamp")
    signature := r.Header.Get("Twitch-Eventsub-Message-Signature")

    if !twitch.VerifyWebhookSignature(messageID, timestamp, signature, body) {
        h.securityLogger.LogWebhookSignatureFailure(r.Context(), r.RemoteAddr)
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Check for duplicate
    if h.webhookProtection.isDuplicate(messageID) {
        w.WriteHeader(http.StatusOK) // Already processed
        return
    }

    // Mark as processed
    h.webhookProtection.markProcessed(messageID)

    // Process webhook...
}
```

---

## Security Logging & Monitoring

### What to Log

**DO Log**:
- Authentication attempts (success and failure)
- Authorization denials
- Rate limit violations
- Webhook signature failures
- Token encryption/decryption failures
- Anomalous activity patterns
- Admin actions (config changes, user management)

**DON'T Log**:
- Plaintext passwords or tokens
- Personally identifiable information (PII)
- Full credit card numbers
- Raw request bodies with sensitive data

**Example**:
```go
// Good: Log security event
securityLogger.LogAuthFailure(ctx, userID, ipAddress, "invalid_password")

// Bad: Log sensitive data
log.Printf("Login failed for user %s with password %s", userID, password) // NEVER DO THIS
```

---

### Log Format

**DO**:
- Use structured JSON logging
- Include timestamp, event type, severity
- Include user ID and IP address (if applicable)
- Include success/failure status
- Use consistent field names

**Example**:
```go
type SecurityEvent struct {
    Timestamp   time.Time              `json:"timestamp"`
    EventType   string                 `json:"event_type"`
    Severity    string                 `json:"severity"`
    UserID      string                 `json:"user_id,omitempty"`
    IPAddress   string                 `json:"ip_address"`
    Success     bool                   `json:"success"`
    Details     map[string]interface{} `json:"details"`
}
```

---

### CloudWatch Alarms

**DO**:
- Set up alarms for critical security events
- Alert on authentication failure spikes (>50 per 5 min)
- Alert on webhook signature failures (>5 per minute)
- Alert on rate limit abuse (>100 per 5 min)
- Alert on permission denial spikes (>20 per 5 min)
- Use SNS topics for notifications

**Example**:
```bash
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-AuthFailures" \
  --metric-name auth_failure \
  --namespace StreamMaxing/Security \
  --statistic Sum \
  --period 300 \
  --threshold 50 \
  --comparison-operator GreaterThanThreshold
```

---

## Dependency Management

### Third-Party Libraries

**DO**:
- Keep dependencies updated
- Run security audits regularly (`go mod audit`)
- Use only well-maintained libraries
- Pin dependency versions in production
- Review security advisories for dependencies
- Remove unused dependencies

**DON'T**:
- Use deprecated libraries
- Ignore security advisories
- Use `latest` tag in production
- Include unnecessary dependencies

**Example**:
```bash
# Check for vulnerabilities
go list -json -m all | nancy sleuth

# Update dependencies
go get -u ./...
go mod tidy

# Audit dependencies
go mod audit
```

---

## Database Security

### Query Safety

**DO**:
- Use parameterized queries ALWAYS
- Use pgx driver with proper escaping
- Validate input before querying
- Use transactions for multi-step operations
- Limit query results (LIMIT clause)

**DON'T**:
- Concatenate user input into SQL
- Use string formatting for queries
- Return full error messages to users
- Allow unlimited result sets

**Example**:
```go
// Good: Parameterized query
query := "SELECT * FROM users WHERE user_id = $1"
db.QueryRow(ctx, query, userID)

// Bad: String concatenation
query := fmt.Sprintf("SELECT * FROM users WHERE user_id = '%s'", userID) // NEVER DO THIS
```

---

### Connection Security

**DO**:
- Use SSL/TLS for database connections (`sslmode=require`)
- Store connection strings in Secrets Manager
- Use connection pooling with limits
- Close connections after use
- Use read-only connections for queries

**DON'T**:
- Use unencrypted connections
- Hard-code connection strings
- Leave connections open indefinitely
- Use root/admin credentials for application

**Example**:
```go
// Good: SSL required
DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require

// Configure pool
config.MaxConns = 10
config.MinConns = 2
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = 30 * time.Minute
```

---

## AWS Security

### IAM Best Practices

**DO**:
- Use IAM roles instead of access keys
- Follow principle of least privilege
- Create service-specific roles
- Enable MFA for admin accounts
- Rotate credentials regularly
- Use IAM policy conditions

**DON'T**:
- Use root account for operations
- Create overly permissive policies
- Share IAM credentials
- Embed credentials in code

**Example**:
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "kms:Decrypt",
      "kms:Encrypt"
    ],
    "Resource": "arn:aws:kms:us-east-1:*:key/alias/streammaxing-oauth"
  }]
}
```

---

### Lambda Security

**DO**:
- Use separate execution roles per function
- Enable AWS X-Ray for tracing
- Set memory and timeout limits
- Use environment variables for config (not secrets)
- Enable CloudWatch Logs
- Use VPC for database access (if needed)

**DON'T**:
- Use shared execution roles
- Store secrets in environment variables
- Allow infinite execution time
- Disable logging
- Use public Lambda endpoints without auth

---

## Incident Response

### When a Security Incident Occurs

1. **Immediate Actions**:
   - Revoke compromised credentials immediately
   - Rotate all secrets in Secrets Manager
   - Invalidate all active sessions
   - Enable additional logging
   - Block suspicious IP addresses

2. **Investigation**:
   - Review security logs and audit trails
   - Identify scope of compromise
   - Check for data exfiltration
   - Document timeline of events

3. **Remediation**:
   - Patch vulnerabilities
   - Update security controls
   - Notify affected users (if required)
   - Report to authorities (if required by law)

4. **Post-Incident**:
   - Conduct post-mortem
   - Update security procedures
   - Implement preventive measures
   - Schedule security audit

---

## Security Checklist for New Features

Before deploying any new feature, verify:

- [ ] All inputs validated and sanitized
- [ ] Authentication required where needed
- [ ] Authorization checks implemented
- [ ] Rate limiting applied
- [ ] Secrets stored in Secrets Manager
- [ ] Sensitive data encrypted at rest
- [ ] Security logging implemented
- [ ] CloudWatch alarms configured
- [ ] Database queries parameterized
- [ ] Error messages don't leak information
- [ ] Dependencies scanned for vulnerabilities
- [ ] HTTPS enforced
- [ ] CORS configured correctly
- [ ] Security testing completed
- [ ] Documentation updated

---

## Regular Security Tasks

### Daily
- Monitor CloudWatch alarms
- Review security event logs
- Check for failed authentication attempts

### Weekly
- Review audit logs for anomalies
- Check rate limit metrics
- Verify backup integrity
- Review access logs

### Monthly
- Rotate JWT secret
- Update dependencies
- Review and update IAM policies
- Conduct security scan
- Clean up revoked sessions table
- Review CloudWatch costs

### Quarterly
- Rotate OAuth secrets
- Conduct penetration testing
- Security audit of codebase
- Review and update security policies
- Update disaster recovery plan

### Annually
- Rotate KMS keys
- Full security assessment
- Update security documentation
- Review compliance requirements
- Conduct tabletop exercises

---

## Tools & Resources

### Security Testing Tools
- **OWASP ZAP**: Web application security scanner
- **Burp Suite**: Security testing platform
- **nancy**: Go dependency vulnerability checker
- **gosec**: Go security checker

### Monitoring Tools
- **AWS CloudWatch**: Logging and monitoring
- **AWS GuardDuty**: Threat detection
- **AWS Inspector**: Security assessment

### Documentation
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [AWS Security Best Practices](https://aws.amazon.com/architecture/security-identity-compliance/)
- [Go Security Best Practices](https://golang.org/doc/security/best-practices)
- [Discord Security Guidelines](https://discord.com/developers/docs/topics/oauth2#security-considerations)
- [Twitch Security](https://dev.twitch.tv/docs/eventsub/handling-webhook-events#security)

---

## Common Security Mistakes to Avoid

1. **Trusting user input** - Always validate
2. **Skipping authorization checks** - Check on every request
3. **Logging sensitive data** - Never log tokens, passwords, PII
4. **Using weak session management** - Use short expiration, revocation
5. **Ignoring rate limiting** - Implement on all endpoints
6. **Storing secrets in code** - Use Secrets Manager
7. **Not encrypting sensitive data** - Encrypt OAuth tokens
8. **Returning detailed errors** - Use generic error messages
9. **Skipping security testing** - Test before every deploy
10. **Not monitoring security events** - Set up alarms and logging

---

## Questions?

If you're unsure about any security practice:
1. Check this SOP first
2. Review OWASP guidelines
3. Consult with security team
4. When in doubt, be more restrictive

**Remember**: Security is not a one-time task, it's an ongoing process. Stay vigilant!
