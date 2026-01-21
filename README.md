# Base App

A modern Go + Vue 3 base project template with PostgreSQL, Keycloak authentication, and Docker Compose orchestration.

## Features

- **Backend**: Go with Gin framework, GORM, JWT authentication
- **Frontend**: Vue 3, TypeScript, Tailwind CSS, Pinia, Vue Query
- **Auth**: Keycloak OIDC integration
- **Database**: PostgreSQL 16
- **Deployment**: Docker Compose with all services

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (for local development)
- Node.js 20+ & pnpm (for local frontend development)

### Running with Docker Compose

```bash
# Clone and navigate to project
cd oneAgent

# Copy environment file
cp .env.example .env

# Start all services
docker-compose up --build

# Access the application
# - App: http://localhost:8080
# - Keycloak Admin: http://localhost:8180 (admin/admin123)
```

### Keycloak Setup (First Time)

1. Access Keycloak at `http://localhost:8180`
2. Login with `admin` / `admin123`
3. Create a new realm named `base-realm`
4. Create a client named `base-app`:
   - Client Protocol: openid-connect
   - Access Type: public
   - Valid Redirect URIs: `http://localhost:8080/*`
   - Web Origins: `http://localhost:8080`
5. Create a test user in the realm

### Local Development

**Backend:**

```bash
cd backend
go mod tidy
go run ./cmd/server
```

**Frontend:**

```bash
cd frontend
pnpm install
pnpm dev
```

## Project Structure

```
oneAgent/
├── backend/
│   ├── cmd/server/          # Application entry point
│   ├── internal/
│   │   ├── config/          # Configuration management
│   │   ├── database/        # PostgreSQL connection
│   │   ├── handler/         # API handlers
│   │   ├── middleware/      # Auth middleware
│   │   └── model/           # Database models
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── api/             # API client
│   │   ├── components/      # Vue components
│   │   ├── composables/     # Vue composables
│   │   ├── router/          # Vue Router
│   │   ├── stores/          # Pinia stores
│   │   └── views/           # Page views
│   └── package.json
├── docker-compose.yml
└── .env.example
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Backend server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `KEYCLOAK_URL` | Keycloak server URL | - |
| `KEYCLOAK_REALM` | Keycloak realm name | `base-realm` |
| `KEYCLOAK_CLIENT_ID` | Keycloak client ID | `base-app` |
| `ENABLE_TRACE` | Enable trace output in chat | `false` |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/api/chat` | Streaming chat (SSE) |
| `GET` | `/api/sessions` | List chat sessions |
| `GET` | `/api/sessions/:id` | Get session with messages |
| `DELETE` | `/api/sessions/:id` | Delete session |
| `GET` | `/api/me` | Get current user info |

## License

MIT
