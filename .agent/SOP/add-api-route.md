# SOP: Add API Route

## Overview
This document describes the standard procedure for adding new API endpoints to the StreamMaxing backend.

---

## Prerequisites
- Backend development environment set up
- Database schema updated (if route requires new tables)
- API design documented

---

## Steps

### 1. Define Route in Router

**File**: `backend/cmd/lambda/main.go`

Add route to the main router:

```go
func setupRouter() *chi.Mux {
    r := chi.NewRouter()

    // ... existing routes ...

    // Add new route
    r.Route("/api/your-resource", func(r chi.Router) {
        r.Use(middleware.AuthMiddleware) // If authentication required
        r.Get("/", yourHandler.List)      // GET /api/your-resource
        r.Post("/", yourHandler.Create)   // POST /api/your-resource
        r.Get("/{id}", yourHandler.Get)   // GET /api/your-resource/{id}
        r.Put("/{id}", yourHandler.Update) // PUT /api/your-resource/{id}
        r.Delete("/{id}", yourHandler.Delete) // DELETE /api/your-resource/{id}
    })

    return r
}
```

---

### 2. Create Handler

**File**: `backend/internal/handlers/your_handler.go`

```go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "your-module/internal/db"
)

type YourHandler struct {
    db *db.YourDB
}

func NewYourHandler(db *db.YourDB) *YourHandler {
    return &YourHandler{db: db}
}

// List returns all resources
func (h *YourHandler) List(w http.ResponseWriter, r *http.Request) {
    // Extract user ID from context (if authenticated)
    userID, ok := r.Context().Value("user_id").(string)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Query database
    resources, err := h.db.ListResources(userID)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // Return JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resources)
}

// Create creates a new resource
func (h *YourHandler) Create(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Name  string `json:"name"`
        Value string `json:"value"`
    }

    // Parse request body
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Validate input
    if input.Name == "" {
        http.Error(w, "Name is required", http.StatusBadRequest)
        return
    }

    // Insert into database
    resource, err := h.db.CreateResource(input.Name, input.Value)
    if err != nil {
        http.Error(w, "Failed to create resource", http.StatusInternalServerError)
        return
    }

    // Return created resource
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(resource)
}

// Get returns a single resource
func (h *YourHandler) Get(w http.ResponseWriter, r *http.Request) {
    resourceID := chi.URLParam(r, "id")

    resource, err := h.db.GetResource(resourceID)
    if err != nil {
        http.Error(w, "Resource not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resource)
}

// Update updates a resource
func (h *YourHandler) Update(w http.ResponseWriter, r *http.Request) {
    resourceID := chi.URLParam(r, "id")

    var input struct {
        Name  string `json:"name"`
        Value string `json:"value"`
    }

    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    if err := h.db.UpdateResource(resourceID, input.Name, input.Value); err != nil {
        http.Error(w, "Failed to update resource", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"message":"Resource updated"}`))
}

// Delete deletes a resource
func (h *YourHandler) Delete(w http.ResponseWriter, r *http.Request) {
    resourceID := chi.URLParam(r, "id")

    if err := h.db.DeleteResource(resourceID); err != nil {
        http.Error(w, "Failed to delete resource", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"message":"Resource deleted"}`))
}
```

---

### 3. Create Database Layer

**File**: `backend/internal/db/your_db.go`

```go
package db

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

type YourDB struct {
    pool *pgxpool.Pool
}

func NewYourDB(pool *pgxpool.Pool) *YourDB {
    return &YourDB{pool: pool}
}

func (db *YourDB) ListResources(userID string) ([]Resource, error) {
    query := `SELECT id, name, value FROM your_table WHERE user_id = $1`

    rows, err := db.pool.Query(context.Background(), query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var resources []Resource
    for rows.Next() {
        var r Resource
        if err := rows.Scan(&r.ID, &r.Name, &r.Value); err != nil {
            return nil, err
        }
        resources = append(resources, r)
    }

    return resources, nil
}

func (db *YourDB) CreateResource(name, value string) (*Resource, error) {
    query := `
        INSERT INTO your_table (name, value)
        VALUES ($1, $2)
        RETURNING id, name, value
    `

    var resource Resource
    err := db.pool.QueryRow(context.Background(), query, name, value).Scan(
        &resource.ID,
        &resource.Name,
        &resource.Value,
    )

    return &resource, err
}

func (db *YourDB) GetResource(id string) (*Resource, error) {
    query := `SELECT id, name, value FROM your_table WHERE id = $1`

    var resource Resource
    err := db.pool.QueryRow(context.Background(), query, id).Scan(
        &resource.ID,
        &resource.Name,
        &resource.Value,
    )

    return &resource, err
}

func (db *YourDB) UpdateResource(id, name, value string) error {
    query := `UPDATE your_table SET name = $2, value = $3 WHERE id = $1`
    _, err := db.pool.Exec(context.Background(), query, id, name, value)
    return err
}

func (db *YourDB) DeleteResource(id string) error {
    query := `DELETE FROM your_table WHERE id = $1`
    _, err := db.pool.Exec(context.Background(), query, id)
    return err
}

type Resource struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Value string `json:"value"`
}
```

---

### 4. Wire Up Dependencies

**File**: `backend/cmd/lambda/main.go`

```go
func main() {
    // ... existing setup ...

    // Initialize database layer
    yourDB := db.NewYourDB(dbPool)

    // Initialize handler
    yourHandler := handlers.NewYourHandler(yourDB)

    // Setup router (already done in step 1)
    r := setupRouter()

    // ... rest of main function ...
}
```

---

### 5. Add Frontend API Client

**File**: `frontend/src/services/api.ts`

```typescript
export async function listResources(): Promise<Resource[]> {
  return fetchAPI('/api/your-resource');
}

export async function createResource(name: string, value: string): Promise<Resource> {
  return fetchAPI('/api/your-resource', {
    method: 'POST',
    body: JSON.stringify({ name, value }),
  });
}

export async function getResource(id: string): Promise<Resource> {
  return fetchAPI(`/api/your-resource/${id}`);
}

export async function updateResource(id: string, name: string, value: string): Promise<void> {
  return fetchAPI(`/api/your-resource/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, value }),
  });
}

export async function deleteResource(id: string): Promise<void> {
  return fetchAPI(`/api/your-resource/${id}`, {
    method: 'DELETE',
  });
}
```

---

### 6. Test the Route

#### Manual Testing

```bash
# Test with curl
curl -X GET http://localhost:3000/api/your-resource \
  -H "Cookie: session=YOUR_JWT_TOKEN"

curl -X POST http://localhost:3000/api/your-resource \
  -H "Content-Type: application/json" \
  -H "Cookie: session=YOUR_JWT_TOKEN" \
  -d '{"name":"test","value":"123"}'
```

#### Unit Testing

**File**: `backend/internal/handlers/your_handler_test.go`

```go
package handlers

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestCreateResource(t *testing.T) {
    handler := NewYourHandler(mockDB)

    body := strings.NewReader(`{"name":"test","value":"123"}`)
    req := httptest.NewRequest("POST", "/api/your-resource", body)
    w := httptest.NewRecorder()

    handler.Create(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("Expected status 201, got %d", w.Code)
    }
}
```

---

## Best Practices

### 1. Input Validation
Always validate user input:
```go
if input.Name == "" || len(input.Name) > 100 {
    http.Error(w, "Invalid name", http.StatusBadRequest)
    return
}
```

### 2. Error Handling
Use consistent error responses:
```go
type ErrorResponse struct {
    Error string `json:"error"`
}

func writeError(w http.ResponseWriter, message string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
```

### 3. Authentication
Use middleware for protected routes:
```go
r.Route("/api/protected", func(r chi.Router) {
    r.Use(middleware.AuthMiddleware)
    r.Get("/", handler.Protected)
})
```

### 4. Authorization
Check user permissions:
```go
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(string)
    resourceID := chi.URLParam(r, "id")

    // Check if user owns this resource
    resource, _ := h.db.GetResource(resourceID)
    if resource.UserID != userID {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Continue with update...
}
```

### 5. CORS
CORS is handled by middleware in main router:
```go
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{os.Getenv("FRONTEND_URL")},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
}))
```

### 6. Logging
Log important operations:
```go
import "log"

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    // ... create logic ...
    log.Printf("Created resource: id=%s, user=%s", resource.ID, userID)
}
```

---

## Deployment

After adding a new route:

1. **Test locally**: Verify route works with `go run cmd/lambda/main.go`
2. **Build**: `GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/lambda/main.go`
3. **Deploy**: `aws lambda update-function-code --function-name streammaxing --zip-file fileb://deployment.zip`
4. **Test in production**: Use CloudWatch Logs to verify

---

## Common Issues

### Issue: 404 Not Found
**Cause**: Route not registered in router
**Fix**: Check `setupRouter()` in `main.go`

### Issue: 401 Unauthorized
**Cause**: Auth middleware blocking request
**Fix**: Ensure JWT cookie is set and valid

### Issue: 500 Internal Server Error
**Cause**: Database query error
**Fix**: Check CloudWatch Logs for error details

### Issue: CORS Error
**Cause**: Frontend origin not allowed
**Fix**: Update CORS configuration in main router

---

## Checklist

- [ ] Route defined in router
- [ ] Handler created with all CRUD methods
- [ ] Database layer implemented
- [ ] Dependencies wired up in main.go
- [ ] Frontend API client added
- [ ] Manual testing completed
- [ ] Unit tests added
- [ ] Error handling implemented
- [ ] Logging added
- [ ] Documentation updated
- [ ] Deployed to Lambda
- [ ] Production testing completed
