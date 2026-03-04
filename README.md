# Image Resizer Microservice

A high-performance image resizing microservice built in Go using Fiber and libvips.

## Features

- **Multi-format support**: WebP, AVIF, JPEG, PNG, JPEG XL
- **Batch Processing**: Generate multiple resolutions and formats from a single source image in one request.
- **Flexible Input**: Process images from a URL or a direct file upload.
- **Lossless WebP**: Support for lossless compression
- **JWT Authentication**: Secure your image processing endpoints
- **SQLite User Store**: Persistent user management using SQLite
- **Resolution-based effort scaling**: Automatically adjusts encoding effort based on image size
- **Constraints for expensive formats**: Configurable limits for AVIF and JPEG XL
- **Docker support**: Easy deployment with Docker
- **Web Interface**: Built-in Preact-based web UI for easy interaction.


## Quick Start

### Using Docker

```bash
# Build and run
docker compose up -d

# Check health
curl http://localhost:8080/health
```

### Local Development

```bash
# Install libvips
# Ubuntu/Debian
sudo apt-get install libvips-dev

# Build
go build -o image-resizer .

# Run interactive setup wizard (optional)
./image-resizer -tui

# Run
./image-resizer
```

## Configuration

Environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | Server port |
| `SERVER_HOST` | 0.0.0.0 | Server host |
| `JWT_SECRET` | your-secret-key | JWT signing secret |
| `JWT_EXPIRY` | 24h | Token expiry time |
| `JWT_REFRESH_EXPIRY` | 7d | Refresh token expiry |
| `ADMIN_USER` | admin | Default admin username |
| `ADMIN_PASSWORD` | admin | Default admin password |
| `REGISTRATION_SECRET` | - | A secret key that must be provided in the `X-Registration-Secret` header to register new users. Registration is disabled if this is not set. |
| `MAX_WIDTH` | 4096 | Maximum image width |
| `MAX_HEIGHT` | 4096 | Maximum image height |
| `IMAGE_TIMEOUT` | 30s | Image download timeout |
| `ALLOWED_HOSTS` | - | Comma-separated allowed image hosts |
| `QUALITY_JPEG` | 80 | Default JPEG quality (1-100) |
| `QUALITY_PNG` | 80 | Default PNG quality (1-100) |
| `QUALITY_WEBP` | 80 | Default WebP quality (1-100) |
| `QUALITY_AVIF` | 60 | Default AVIF quality (1-100) |
| `QUALITY_JXL` | 75 | Default JPEG XL quality (1-100) |
| `AVIF_MAX_RESOLUTION` | 2048 | Max width/height for AVIF |
| `JXL_MAX_RESOLUTION` | 1920 | Max width/height for JPEG XL |
| `AVIF_MAX_PIXELS` | 2500000 | Max total pixels for AVIF |
| `JXL_MAX_PIXELS` | 2000000 | Max total pixels for JPEG XL |
| `ENABLE_AVIF` | true | Enable AVIF format |
| `ENABLE_JXL` | true | Enable JPEG XL format |


## API Endpoints

### Authentication

#### POST /auth/register 

Register a new user. This endpoint requires a matching `X-Registration-Secret` header, which is configured on the server via the `REGISTRATION_SECRET` environment variable.

**Request:**
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -H "X-Registration-Secret: your-secret-key" \
  -d '{"username":"newuser"}'
```

**Response (Success):**
```json
{
  "message": "User registered successfully",
  "user": "newuser",
  "passkey": "generated-passkey-string"
}
```

#### POST /auth/login

Login to receive JWT token. You can use either your password or the generated passkey.

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
```

Response:
```json
{
  "token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_at": 1772696891,
  "user": {
    "id": "1",
    "username": "admin",
    "email": "admin@example.com"
  }
}
```

#### POST /auth/refresh

Refresh an expired token.

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"eyJhbGc..."}'
```

### Image Processing

#### GET /resize

Resize and convert an image. Requires JWT authentication.

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Source image URL (required) |
| `width` | int | Target width (optional) |
| `height` | int | Target height (optional) |
| `format` | string | Output format: webp, avif, jpg, png, jxl |
| `quality` | int | Output quality 1-100 (optional) |
| `lossless` | bool | Use lossless compression for WebP |

**Example:**

```bash
# Basic resize
curl "http://localhost:8080/resize?url=https://example.com/image.jpg&width=200&height=200" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o output.jpg

# Convert to WebP
curl "http://localhost:8080/resize?url=https://example.com/image.jpg&width=300&format=webp" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o output.webp

# Lossless WebP
curl "http://localhost:8080/resize?url=https://example.com/image.png&width=200&format=webp&lossless=true" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o output.webp

# Convert to AVIF
curl "http://localhost:8080/resize?url=https://example.com/image.jpg&width=400&format=avif" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o output.avif
```

### Health Check

#### GET /health

Check service health and configuration.

```bash
curl http://localhost:8080/health
```

## Effort Scaling

The service automatically adjusts encoding effort based on image resolution:

| Pixels | Effort |
|--------|--------|
| < 500K | 6 (best compression) |
| < 1M | 5 |
| < 2M | 4 |
| < 4M | 3 |
| >= 4M | 2 (fastest) |

## Constraints

AVIF and JPEG XL are computationally expensive. The service enforces:

- **AVIF**: Max dimension 2048px, max 2.5MP (configurable)
- **JPEG XL**: Max dimension 1920px, max 2MP (configurable)

Requests exceeding these limits return an error.

## Error Codes

| Code | Description |
|------|-------------|
| 1001 | URL parameter required |
| 1002 | Invalid URL format |
| 1003 | Host not allowed |
| 1004 | Failed to download image |
| 1005 | Failed to decode image |
| 1006 | Failed to resize image |
| 1007 | Failed to export image |
| 1008 | Constraint violation (AVIF/JXL limits) |

## Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o image-resizer .
```

## License

MIT
