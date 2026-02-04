# Security Guide

This document covers security features in Iris, including API key management, the encrypted keystore, and best practices for production deployments.

## Keystore Encryption

Iris provides encrypted storage for API keys using AES-256-GCM encryption. The keystore supports two formats:

| Format | Key Derivation | Security Level | Use Case |
|--------|----------------|----------------|----------|
| V1 (Legacy) | SHA-256 of machine data | Basic | Development, single-user machines |
| V2 | Argon2id with user master key | Strong | Production, shared machines, CI/CD |

### Creating an Encryption Key (Recommended)

For production use, set the `IRIS_KEYSTORE_KEY` environment variable with a strong, unique passphrase:

```bash
# Generate a strong random key (recommended)
export IRIS_KEYSTORE_KEY=$(openssl rand -base64 32)

# Or use a memorable passphrase (minimum 16 characters recommended)
export IRIS_KEYSTORE_KEY="your-strong-passphrase-here"
```

Add this to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.) for persistence:

```bash
# ~/.bashrc or ~/.zshrc
export IRIS_KEYSTORE_KEY="your-strong-passphrase-here"
```

### How It Works

When `IRIS_KEYSTORE_KEY` is set, Iris uses the V2 keystore format:

1. **Key Derivation**: Your master key is processed through Argon2id with OWASP-recommended parameters:
   - Time cost: 3 iterations
   - Memory cost: 64 MB
   - Parallelism: 4 threads
   - Output: 256-bit encryption key

2. **Encryption**: Data is encrypted using AES-256-GCM with:
   - Random 16-byte salt (per-file)
   - Random 12-byte nonce (per-write)
   - Authenticated encryption (tamper detection)

3. **File Format**: `[IRIS magic header][version][salt][nonce][ciphertext]`

### Storing API Keys

Once your encryption key is configured, store your provider API keys:

```bash
# Set API keys (prompts for key without echo)
iris keys set openai
iris keys set anthropic
iris keys set gemini

# List stored keys (shows provider names only, never values)
iris keys list

# Delete a key
iris keys delete openai
```

Keys are stored at `~/.iris/keys.enc` with restrictive permissions (0600).

### V1 Legacy Mode

If `IRIS_KEYSTORE_KEY` is not set, Iris falls back to V1 mode which derives the encryption key from:
- Machine hostname
- Current username
- A static salt

**Security Note**: V1 is convenient for development but the key is predictable. Anyone with access to your machine can derive the same key. Use V2 with `IRIS_KEYSTORE_KEY` for production.

### Migrating from V1 to V2

Existing V1 keystores are automatically read when you set `IRIS_KEYSTORE_KEY`. On the next write operation, the keystore is upgraded to V2 format. A backup of the V1 file is created at `~/.iris/keys.enc.v1.bak`.

To manually migrate:

```go
import "github.com/petal-labs/iris/cli/keystore"

ks, _ := keystore.NewFileKeystoreWithSource(
    keystore.DefaultKeystorePath(),
    &keystore.EnvMasterKeySource{},
)
ks.MigrateToV2()
```

## Secret Type

Iris uses a `core.Secret` type for API keys to prevent accidental exposure:

```go
import "github.com/petal-labs/iris/core"

secret := core.NewSecret("sk-abc123...")

// Safe: these return "[REDACTED]"
fmt.Println(secret)           // [REDACTED]
fmt.Printf("%s", secret)      // [REDACTED]
fmt.Printf("%v", secret)      // [REDACTED]
fmt.Printf("%#v", secret)     // core.Secret{REDACTED}
json.Marshal(secret)          // "[REDACTED]"

// Explicit access when needed
apiKey := secret.Expose()     // "sk-abc123..."

// Safe emptiness check
if secret.IsEmpty() {
    log.Fatal("API key required")
}
```

All providers accept `core.Secret` for API keys:

```go
// From environment variable
secret := core.NewSecret(os.Getenv("OPENAI_API_KEY"))
provider := openai.New(secret)

// Or use the string convenience (converts internally)
provider := openai.New(os.Getenv("OPENAI_API_KEY"))
```

## Telemetry Security

When implementing telemetry hooks, be careful not to log sensitive data:

```go
type SafeTelemetry struct{}

func (t SafeTelemetry) OnRequestStart(e core.RequestStartEvent) {
    // Safe: Model and Provider are not sensitive
    log.Printf("Request to %s/%s", e.Provider, e.Model)
}

func (t SafeTelemetry) OnRequestEnd(e core.RequestEndEvent) {
    // Safe: Usage stats are not sensitive
    log.Printf("Completed: %d tokens", e.Usage.TotalTokens)

    // UNSAFE: Never log request/response content in production
    // log.Printf("Response: %s", resp.Output)  // DON'T DO THIS
}
```

## CI/CD Best Practices

### GitHub Actions

Store your keystore master key and API keys as repository secrets:

```yaml
# .github/workflows/ci.yml
env:
  IRIS_KEYSTORE_KEY: ${{ secrets.IRIS_KEYSTORE_KEY }}
  OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Environment-Specific Keys

Use different master keys for different environments:

```bash
# Development
export IRIS_KEYSTORE_KEY="dev-key-not-for-production"

# Staging
export IRIS_KEYSTORE_KEY="$STAGING_KEYSTORE_KEY"

# Production
export IRIS_KEYSTORE_KEY="$PRODUCTION_KEYSTORE_KEY"
```

### Docker

Pass the keystore key at runtime, never bake it into images:

```dockerfile
# Dockerfile
FROM golang:1.24-alpine
# ... build steps ...
# DO NOT: ENV IRIS_KEYSTORE_KEY=...
```

```bash
# Run with key
docker run -e IRIS_KEYSTORE_KEY="$IRIS_KEYSTORE_KEY" myapp
```

## Security Checklist

### Development

- [ ] Set `IRIS_KEYSTORE_KEY` in your shell profile
- [ ] Use `iris keys set <provider>` instead of environment variables when possible
- [ ] Never commit `.iris/` directory or `keys.enc` files

### Production

- [ ] Use a strong, unique `IRIS_KEYSTORE_KEY` (32+ random bytes recommended)
- [ ] Store secrets in a secure secret manager (Vault, AWS Secrets Manager, etc.)
- [ ] Rotate API keys periodically
- [ ] Use separate keystores per environment
- [ ] Enable audit logging for key access
- [ ] Review telemetry hooks for sensitive data leakage

### Code Review

- [ ] Ensure `core.Secret` is used for all API keys
- [ ] Check that `secret.Expose()` is only called when necessary
- [ ] Verify no sensitive data in log statements
- [ ] Confirm error messages don't leak secrets

## Cryptographic Details

### Argon2id Parameters (V2)

The V2 keystore uses Argon2id with OWASP-recommended parameters:

| Parameter | Value | Purpose |
|-----------|-------|---------|
| Time | 3 iterations | Increases computation cost |
| Memory | 64 MB | Makes GPU attacks expensive |
| Parallelism | 4 threads | Utilizes multi-core CPUs |
| Salt | 16 bytes (random) | Prevents rainbow tables |
| Output | 32 bytes | 256-bit AES key |

### AES-256-GCM

| Property | Value |
|----------|-------|
| Algorithm | AES-256-GCM |
| Key size | 256 bits |
| Nonce size | 96 bits (12 bytes) |
| Tag size | 128 bits |
| Mode | Authenticated encryption |

### File Permissions

| File | Permissions | Purpose |
|------|-------------|---------|
| `~/.iris/` | 0700 | Directory readable only by owner |
| `~/.iris/keys.enc` | 0600 | Keystore readable only by owner |
| `~/.iris/keys.enc.v1.bak` | 0600 | V1 backup (if migrated) |

## Reporting Security Issues

If you discover a security vulnerability in Iris, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email security concerns to the maintainers
3. Include steps to reproduce the issue
4. Allow time for a fix before public disclosure
