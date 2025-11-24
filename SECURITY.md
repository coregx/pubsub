# Security Policy

## Supported Versions

PubSub-Go Library is currently in pre-release development. We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| Pre-release (main/develop) | :white_check_mark: |

Future stable releases (v1.0+) will follow semantic versioning with LTS support.

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in PubSub-Go Library, please report it responsibly.

### How to Report

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues by:

1. **Private Security Advisory** (preferred):
   https://github.com/coregx/pubsub/security/advisories/new

2. **Email** to maintainers:
   Create a private GitHub issue or contact via discussions

### What to Include

Please include the following information in your report:

- **Description** of the vulnerability
- **Steps to reproduce** the issue
- **Affected versions** (which versions are impacted)
- **Potential impact** (DoS, data breach, SQL injection, etc.)
- **Suggested fix** (if you have one)
- **Your contact information** (for follow-up questions)

### Response Timeline

- **Initial Response**: Within 48-72 hours
- **Triage & Assessment**: Within 1 week
- **Fix & Disclosure**: Coordinated with reporter

We aim to:
1. Acknowledge receipt within 72 hours
2. Provide an initial assessment within 1 week
3. Work with you on a coordinated disclosure timeline
4. Credit you in the security advisory (unless you prefer to remain anonymous)

## Security Considerations for Pub/Sub Systems

Pub/Sub systems handle message routing and delivery, which introduces security risks.

### 1. SQL Injection

**Risk**: Database queries with unsanitized user input.

**Attack Vectors**:
- Topic codes with SQL injection payloads
- Subscription identifiers with malicious strings
- Publisher/subscriber names with SQL commands

**Mitigation in Library**:
- ✅ Relica query builder prevents SQL injection
- ✅ Parameterized queries throughout
- ✅ Input validation for all user-provided strings
- ✅ No string concatenation in SQL

**User Recommendations**:
```go
// ✅ SAFE - Library uses Relica query builder
publisher.Publish(ctx, pubsub.PublishRequest{
    TopicCode:  userInput,  // Safe - parameterized query
    Identifier: identifier, // Safe - bound parameter
    Data:       data,       // Safe - stored as-is
})

// ❌ DO NOT bypass library and use raw SQL
// db.Exec("INSERT INTO messages WHERE topic = '" + userInput + "'")
```

### 2. Webhook Security

**Risk**: Subscriber webhook URLs can be abused for SSRF attacks.

**Attack Vectors**:
- Internal network URLs (http://localhost:8080)
- Cloud metadata endpoints (http://169.254.169.254)
- File protocol URLs (file:///etc/passwd)

**Mitigation**:
- 🔄 **TODO (v0.2.0)**: Webhook URL validation
- 🔄 **TODO (v0.2.0)**: Blacklist internal IPs and metadata endpoints
- 🔄 **TODO (v0.2.0)**: HTTPS-only enforcement option
- 🔄 **TODO (v0.2.0)**: Webhook signature verification

**Current Recommendations**:
```go
// ❌ DANGEROUS - Accept any webhook URL
subscriber := model.NewSubscriber(clientId, "Test", "http://localhost:8080/hook")

// ✅ BETTER - Validate webhook URLs before creating subscribers
func ValidateWebhookURL(url string) error {
    parsed, err := url.Parse(url)
    if err != nil {
        return err
    }

    // Require HTTPS
    if parsed.Scheme != "https" {
        return errors.New("webhook must use HTTPS")
    }

    // Block private networks
    ip := net.ParseIP(parsed.Hostname())
    if ip != nil && ip.IsPrivate() {
        return errors.New("webhook cannot target private network")
    }

    return nil
}
```

### 3. Message Injection

**Risk**: Malicious message content can exploit subscriber systems.

**Attack Vectors**:
- XSS payloads in message data
- Script injection in JSON fields
- Command injection via message parameters

**Mitigation**:
- ✅ Library stores messages as-is (no interpretation)
- ✅ Subscribers responsible for sanitizing received data
- ⚠️ **User Responsibility**: Validate and sanitize message content

**Subscriber Best Practices**:
```go
// ❌ DANGEROUS - Direct use of message data
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
    var msg pubsub.DataMessage
    json.NewDecoder(r.Body).Decode(&msg)

    // NEVER do this!
    fmt.Fprintf(w, "<div>%s</div>", msg.Data) // XSS!
}

// ✅ SAFE - Sanitize and validate
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
    var msg pubsub.DataMessage
    json.NewDecoder(r.Body).Decode(&msg)

    // Validate structure
    if !isValidMessageData(msg.Data) {
        http.Error(w, "invalid message", http.StatusBadRequest)
        return
    }

    // Sanitize before use
    safeData := html.EscapeString(msg.Data)
    fmt.Fprintf(w, "<div>%s</div>", safeData)
}
```

### 4. Denial of Service (DoS)

**Risk**: Message flooding can exhaust resources.

**Attack Vectors**:
- Massive message publishing
- Extremely large message payloads
- Queue exhaustion via failed deliveries

**Mitigation**:
- 🔄 **TODO (v0.2.0)**: Rate limiting per publisher
- 🔄 **TODO (v0.2.0)**: Message size limits
- 🔄 **TODO (v0.2.0)**: Queue size monitoring
- ✅ DLQ prevents infinite retries
- ✅ Exponential backoff prevents tight retry loops

**Current Recommendations**:
```go
// Application-level rate limiting
type RateLimiter struct {
    publishers map[int64]*rate.Limiter
    mu         sync.RWMutex
}

func (r *RateLimiter) Allow(publisherId int64) bool {
    r.mu.RLock()
    limiter := r.publishers[publisherId]
    r.mu.RUnlock()

    if limiter == nil {
        limiter = rate.NewLimiter(rate.Limit(100), 200) // 100/sec, burst 200
        r.mu.Lock()
        r.publishers[publisherId] = limiter
        r.mu.Unlock()
    }

    return limiter.Allow()
}
```

### 5. Authentication & Authorization

**Risk**: Unauthorized access to publishing or subscription management.

**Mitigation**:
- ⚠️ **No built-in authentication** - Library focuses on core pub/sub logic
- ⚠️ **User Responsibility**: Implement authentication in your application layer

**Recommended Patterns**:
```go
// REST API with authentication
func PublishHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Authenticate request
    user, err := authenticateRequest(r)
    if err != nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Authorize publisher access
    if !canPublishToTopic(user, topicCode) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }

    // 3. Use library safely
    result, err := publisher.Publish(ctx, req)
    // ...
}
```

### 6. Data Persistence Security

**Risk**: Database compromise exposes all messages.

**Mitigation**:
- 🔄 **TODO (v0.2.0)**: Message encryption at rest
- ⚠️ **User Responsibility**: Database access control
- ⚠️ **User Responsibility**: Network security

**Database Security Checklist**:
- [ ] Use strong database passwords
- [ ] Enable TLS for database connections
- [ ] Restrict database access to application servers only
- [ ] Regular database backups
- [ ] Encrypt sensitive data before publishing

```go
// Encrypt sensitive data before publishing
func PublishSensitiveData(ctx context.Context, publisher *pubsub.Publisher, data SensitiveData) error {
    // 1. Serialize data
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }

    // 2. Encrypt with your key management
    encrypted, err := encryptWithKMS(jsonData)
    if err != nil {
        return err
    }

    // 3. Publish encrypted data
    return publisher.Publish(ctx, pubsub.PublishRequest{
        TopicCode:  "sensitive.data",
        Identifier: data.ID,
        Data:       base64.StdEncoding.EncodeToString(encrypted),
    })
}
```

## Security Best Practices for Users

### Input Validation

Always validate user input before creating subscriptions or publishing:

```go
// Validate topic codes
func ValidateTopicCode(code string) error {
    if len(code) == 0 || len(code) > 255 {
        return errors.New("invalid topic code length")
    }

    // Allow only alphanumeric, dots, dashes
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9.-]+$`, code)
    if !matched {
        return errors.New("invalid topic code format")
    }

    return nil
}
```

### Error Handling

Never expose internal errors to end users:

```go
// ❌ BAD - Exposes internal details
func PublishHandler(w http.ResponseWriter, r *http.Request) {
    _, err := publisher.Publish(ctx, req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError) // Leak!
    }
}

// ✅ GOOD - Generic error messages
func PublishHandler(w http.ResponseWriter, r *http.Request) {
    _, err := publisher.Publish(ctx, req)
    if err != nil {
        log.Printf("Publish failed: %v", err) // Log details
        http.Error(w, "failed to publish message", http.StatusInternalServerError)
    }
}
```

### Logging

Log security-relevant events but avoid logging sensitive data:

```go
// ✅ GOOD - Log actions without sensitive data
logger.Infof("Publisher %d published to topic %s (identifier: %s)",
    publisherId, topicCode, identifier)

// ❌ BAD - Logging message content
logger.Infof("Published message: %s", messageData) // Could contain PII!
```

## Known Security Considerations

### 1. No Message Encryption

**Status**: Planned for v0.2.0.

**Risk Level**: Medium

**Description**: Messages are stored in plain text in the database. Compromised database = exposed messages.

**Mitigation**:
- ⚠️ **User Responsibility**: Encrypt sensitive data before publishing
- 🔄 **TODO (v0.2.0)**: Built-in message encryption option

### 2. No Webhook Verification

**Status**: Planned for v0.2.0.

**Risk Level**: Medium

**Description**: No built-in mechanism to verify subscriber webhooks authentically belong to claimed subscribers.

**Mitigation**:
- 🔄 **TODO (v0.2.0)**: HMAC signature verification
- ⚠️ **User Responsibility**: Verify webhook origins in subscriber code

### 3. No Rate Limiting

**Status**: Planned for v0.2.0.

**Risk Level**: Medium to High

**Description**: No built-in rate limiting. Publishers can flood the system.

**Mitigation**:
- ⚠️ **User Responsibility**: Implement application-level rate limiting
- 🔄 **TODO (v0.2.0)**: Built-in rate limiting per publisher

### 4. Dependency Security

PubSub-Go Library has minimal dependencies:

- `github.com/coregx/relica` - Query builder (v0.6.0)
- `github.com/go-ozzo/ozzo-validation/v4` - Input validation
- `github.com/go-sql-driver/mysql` - MySQL driver (optional)
- `github.com/lib/pq` - PostgreSQL driver (optional)
- `github.com/mattn/go-sqlite3` - SQLite driver (optional)
- `github.com/stretchr/testify` - Testing (dev only)

**Monitoring**:
- 🔄 Dependabot enabled (when repository goes public)
- 🔄 Weekly dependency audit (planned)
- ✅ Pure Go (no C dependencies except SQLite)

## Security Testing

### Current Testing

- ✅ Unit tests with edge cases
- ✅ Integration tests with real databases
- ✅ Linting with 34+ security-focused linters
- ✅ Input validation tests

### Planned for v1.0

- 🔄 Fuzzing with go-fuzz
- 🔄 Static analysis with gosec
- 🔄 SAST/DAST scanning in CI
- 🔄 Penetration testing
- 🔄 Security audit

## Security Disclosure History

No security vulnerabilities have been reported or fixed yet (project is in pre-release).

When vulnerabilities are addressed, they will be listed here with:
- **CVE ID** (if assigned)
- **Affected versions**
- **Fixed in version**
- **Severity** (Critical/High/Medium/Low)
- **Credit** to reporter

## Security Contact

- **GitHub Security Advisory**: https://github.com/coregx/pubsub/security/advisories/new
- **Public Issues** (for non-sensitive bugs): https://github.com/coregx/pubsub/issues
- **Discussions**: https://github.com/coregx/pubsub/discussions

## Bug Bounty Program

PubSub-Go Library does not currently have a bug bounty program. We rely on responsible disclosure from the security community.

If you report a valid security vulnerability:
- ✅ Public credit in security advisory (if desired)
- ✅ Acknowledgment in CHANGELOG
- ✅ Our gratitude and recognition in README
- ✅ Priority review and quick fix

---

**Thank you for helping keep PubSub-Go Library secure!** 🔒

*Security is a journey, not a destination. We continuously improve our security posture with each release.*
