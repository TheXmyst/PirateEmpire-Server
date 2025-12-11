# Security Improvements Summary

This document summarizes the security improvements made to the Sea-Dogs backend.

## Changes Made

### 1. Password Hashing (BCrypt)
- **File**: `server/internal/auth/password.go`
- **Changes**:
  - Added `HashPassword()` function to hash passwords using bcrypt
  - Added `CheckPasswordHash()` function to verify passwords
  - Modified `Register()` handler to hash passwords before storing
  - Modified `Login()` handler to verify passwords using bcrypt
  - Added legacy support: existing plaintext passwords are auto-upgraded to hashed on first login

### 2. Token-Based Authentication
- **Files**: 
  - `server/internal/auth/token.go` - Token generation and validation
  - `server/internal/auth/middleware.go` - Authentication middleware
- **Changes**:
  - Implemented simple token-based authentication using HMAC-SHA256
  - Tokens expire after 24 hours
  - Token secret can be set via `AUTH_SECRET` environment variable
  - Added `RequireAuth` middleware for protecting endpoints
  - Login response now includes optional `token` field for Bearer authentication

### 3. Input Validation
- **File**: `server/internal/api/handlers.go`
- **Changes**:
  - Added validation for username and password in `Register()` and `Login()`
  - Username and password are trimmed of whitespace
  - Password must be at least 6 characters
  - Returns HTTP 400 with clear error messages on validation failure

### 4. Admin Configuration
- **File**: `server/internal/api/handlers.go`
- **Changes**:
  - Admin username can now be configured via `ADMIN_USERNAME` environment variable
  - Falls back to hardcoded "TheXmyst" for backward compatibility
  - Admin password is automatically hashed on first login

### 5. Protected Endpoints
- **File**: `server/cmd/main.go`
- **Changes**:
  - Applied `RequireAuth` middleware to all game action endpoints:
    - `/build`, `/upgrade`, `/research/start`, `/reset`
    - `/add-resources`, `/build-ship`
    - `/fleets/*` endpoints
    - `/dev/*` endpoints
  - Public endpoints remain unprotected:
    - `/register`, `/login`, `/status`, `/health`

### 6. Backward Compatibility
- **Maintained**:
  - All existing API endpoints remain functional
  - `player_id` in request body/query params still works (fallback)
  - Token authentication is optional (clients can still use `player_id`)
  - Login response includes `token` as optional field (non-breaking)

### 7. Test Coverage
- **Files**:
  - `server/internal/auth/password_test.go`
  - `server/internal/auth/token_test.go`
- **Coverage**:
  - Password hashing and verification
  - Token generation and validation
  - Error cases (invalid tokens, wrong passwords)

## Environment Variables

### AUTH_SECRET (Optional)
- Secret key for token signing
- If not set, a random secret is generated (not persistent across restarts)
- **Recommended**: Set this in production for consistent token validation
- Minimum length: 32 characters recommended

### ADMIN_USERNAME (Optional)
- Username that should have admin privileges
- If not set, defaults to "TheXmyst" for backward compatibility
- **Recommended**: Set this in production to use a different admin username

## Migration Notes

### Existing Users
- Users with plaintext passwords can still login
- Passwords are automatically hashed on first successful login
- No manual migration required

### Database
- Existing plaintext passwords in database will continue to work
- They will be automatically upgraded to hashed format on next login
- For production, consider forcing password reset for all users

### Client Updates
- Clients can continue using `player_id` in requests (backward compatible)
- Clients can optionally use Bearer token authentication:
  - Send `Authorization: Bearer <token>` header
  - Token is returned in login response as `token` field
- Token provides better security and doesn't require sending `player_id` in every request

## Security Considerations

### Current Implementation
- ✅ Passwords are hashed using bcrypt
- ✅ Token-based authentication available
- ✅ Input validation on auth endpoints
- ✅ Protected endpoints require authentication
- ✅ Legacy plaintext password support (for migration)

### Recommendations for Production
1. **Set AUTH_SECRET environment variable** - Use a strong, random secret (32+ characters)
2. **Set ADMIN_USERNAME environment variable** - Don't rely on hardcoded admin username
3. **Force password reset** - After deployment, force all users to reset passwords
4. **Remove legacy password support** - Once all passwords are hashed, remove plaintext fallback
5. **Use HTTPS** - All authentication should happen over HTTPS
6. **Rate limiting** - Consider adding rate limiting to login/register endpoints
7. **Token refresh** - Consider implementing token refresh mechanism for longer sessions

## Testing

Run tests with:
```bash
cd server
go test ./internal/auth/...
```

All tests should pass. The test suite covers:
- Password hashing and verification
- Token generation and validation
- Error handling

## Files Modified

### New Files
- `server/internal/auth/password.go`
- `server/internal/auth/token.go`
- `server/internal/auth/middleware.go`
- `server/internal/auth/password_test.go`
- `server/internal/auth/token_test.go`

### Modified Files
- `server/internal/api/handlers.go` - Added password hashing, validation, token generation
- `server/cmd/main.go` - Applied auth middleware to protected routes

### Unchanged (Backward Compatible)
- All game logic, economy formulas, and JSON configs remain unchanged
- All API response formats remain backward compatible
- Client code continues to work without modifications

