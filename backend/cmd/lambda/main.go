package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/handlers"
	"github.com/yourusername/streammaxing/internal/middleware"
	"github.com/yourusername/streammaxing/internal/services/discord"
	"github.com/yourusername/streammaxing/internal/services/notifications"
	"github.com/yourusername/streammaxing/internal/services/twitch"
)

// Router maps HTTP routes to handlers with path parameter extraction
type Router struct {
	routes []route
}

type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{}
}

// Handle registers a handler for a method and path pattern
func (router *Router) Handle(method, pattern string, handler http.HandlerFunc) {
	router.routes = append(router.routes, route{
		method:  method,
		pattern: pattern,
		handler: handler,
	})
}

// ServeHTTP handles incoming HTTP requests
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rt := range router.routes {
		if rt.method != r.Method {
			continue
		}
		params, ok := matchPath(rt.pattern, r.URL.Path)
		if ok {
			// Store path params in context
			ctx := r.Context()
			for k, v := range params {
				ctx = context.WithValue(ctx, pathParamKey(k), v)
			}
			rt.handler(w, r.WithContext(ctx))
			return
		}
	}
	http.Error(w, "Not found", http.StatusNotFound)
}

type pathParamKey string

// getPathParam extracts a path parameter from the request context
func getPathParam(r *http.Request, name string) string {
	v, _ := r.Context().Value(pathParamKey(name)).(string)
	return v
}

// matchPath checks if a route pattern matches a path and extracts parameters
func matchPath(pattern, path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			params[part[1:]] = pathParts[i]
		} else if part != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

// setupRoutes configures all API routes
func setupRoutes(router *Router) {
	// Initialize services
	twitchAPIClient := twitch.NewAPIClient()
	discordAPIClient := discord.NewAPIClient()
	fanoutService := notifications.NewFanoutService(twitchAPIClient, discordAPIClient)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler()
	guildHandler := handlers.NewGuildHandler()
	twitchAuthHandler := handlers.NewTwitchAuthHandler(twitchAPIClient)
	webhookHandler := handlers.NewWebhookHandler(fanoutService)
	preferencesHandler := handlers.NewPreferencesHandler()
	inviteHandler := handlers.NewInviteHandler()

	// ==================
	// Public routes
	// ==================

	// Health check
	router.Handle("GET", "/api/health", healthHandler)

	// Discord OAuth (no auth required)
	router.Handle("GET", "/api/auth/discord/login", authHandler.DiscordLogin)
	router.Handle("GET", "/api/auth/discord/callback", authHandler.DiscordCallback)

	// Twitch OAuth callback (auth required - user must be logged in)
	router.Handle("GET", "/api/auth/twitch/callback", middleware.AuthMiddleware(twitchAuthHandler.TwitchCallback))

	// Webhook endpoint (signature verification, no JWT auth)
	router.Handle("POST", "/webhooks/twitch", webhookHandler.HandleTwitchWebhook)

	// ==================
	// Authenticated routes
	// ==================

	// Auth
	router.Handle("POST", "/api/auth/logout", middleware.AuthMiddleware(authHandler.Logout))
	router.Handle("GET", "/api/auth/me", middleware.AuthMiddleware(authHandler.GetMe))

	// Guilds
	router.Handle("GET", "/api/guilds", middleware.AuthMiddleware(guildHandler.GetUserGuilds))

	router.Handle("GET", "/api/guilds/:guild_id/channels", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.GetGuildChannels(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("GET", "/api/guilds/:guild_id/roles", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.GetGuildRoles(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("GET", "/api/guilds/:guild_id/streamers", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.GetGuildStreamers(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("DELETE", "/api/guilds/:guild_id/streamers/:streamer_id", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.UnlinkStreamer(w, r, getPathParam(r, "guild_id"), getPathParam(r, "streamer_id"))
	}))

	router.Handle("GET", "/api/guilds/:guild_id/streamers/link", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		twitchAuthHandler.InitiateStreamerLink(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("GET", "/api/guilds/:guild_id/config", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.GetGuildConfig(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("PUT", "/api/guilds/:guild_id/config", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.UpdateGuildConfig(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("GET", "/api/guilds/:guild_id/bot-install-url", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.GetBotInstallURL(w, r, getPathParam(r, "guild_id"))
	}))

	// Streamer message (custom notification text)
	router.Handle("GET", "/api/guilds/:guild_id/streamers/:streamer_id/message", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.GetStreamerMessage(w, r, getPathParam(r, "guild_id"), getPathParam(r, "streamer_id"))
	}))

	router.Handle("PUT", "/api/guilds/:guild_id/streamers/:streamer_id/message", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		guildHandler.UpdateStreamerMessage(w, r, getPathParam(r, "guild_id"), getPathParam(r, "streamer_id"))
	}))

	// Invite links (admin)
	router.Handle("POST", "/api/guilds/:guild_id/invites", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		inviteHandler.CreateInvite(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("GET", "/api/guilds/:guild_id/invites", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		inviteHandler.ListInvites(w, r, getPathParam(r, "guild_id"))
	}))

	router.Handle("DELETE", "/api/guilds/:guild_id/invites/:invite_id", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		inviteHandler.DeleteInvite(w, r, getPathParam(r, "guild_id"), getPathParam(r, "invite_id"))
	}))

	// Invite links (public / any user)
	router.Handle("GET", "/api/invites/:code", func(w http.ResponseWriter, r *http.Request) {
		inviteHandler.GetInviteInfo(w, r, getPathParam(r, "code"))
	})
	router.Handle("POST", "/api/invites/:code/accept", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		inviteHandler.AcceptInvite(w, r, getPathParam(r, "code"))
	}))

	// User preferences
	router.Handle("GET", "/api/users/me/preferences", middleware.AuthMiddleware(preferencesHandler.GetUserPreferences))

	router.Handle("PUT", "/api/users/me/preferences/:guild_id/:streamer_id", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		preferencesHandler.UpdateUserPreference(w, r, getPathParam(r, "guild_id"), getPathParam(r, "streamer_id"))
	}))
}

// healthHandler returns API health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":  "ok",
		"message": "StreamMaxing API v3",
		"version": "3.0.0",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handler is the Lambda function handler (API Gateway HTTP API v2 payload format)
func Handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Initialize database connection (if not already connected)
	if db.Pool == nil {
		if err := db.Connect(); err != nil {
			log.Printf("Failed to connect to database: %v", err)
			return events.APIGatewayV2HTTPResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       `{"error": "Database connection failed"}`,
			}, nil
		}
	}

	// Create router
	router := NewRouter()
	setupRoutes(router)

	// Debug: log raw request info for auth callbacks
	if strings.Contains(request.RawPath, "callback") {
		log.Printf("[LAMBDA_DEBUG] RawPath=%s Cookies=%v HeaderCookie=%q",
			request.RawPath, request.Cookies, request.Headers["cookie"])
	}

	// Convert API Gateway v2 request to http.Request
	httpReq, err := convertAPIGatewayV2Request(request)
	if err != nil {
		log.Printf("Failed to convert request: %v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "Internal server error"}`,
		}, nil
	}

	// Create response writer
	rw := newResponseWriter()

	// Apply CORS middleware
	handler := middleware.CORSMiddleware(router.ServeHTTP)

	// Serve request
	handler(rw, httpReq)

	// Convert response headers: separate Set-Cookie into the Cookies field
	respHeaders := make(map[string]string)
	var cookies []string
	for key, values := range rw.headers {
		if strings.EqualFold(key, "Set-Cookie") {
			cookies = append(cookies, values...)
		} else if len(values) > 0 {
			respHeaders[key] = values[len(values)-1]
		}
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: rw.statusCode,
		Headers:    respHeaders,
		Cookies:    cookies,
		Body:       rw.body.String(),
	}, nil
}

// convertAPIGatewayV2Request converts API Gateway v2 HTTP request to http.Request
func convertAPIGatewayV2Request(req events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	method := req.RequestContext.HTTP.Method
	path := req.RawPath
	if path == "" {
		path = req.RequestContext.HTTP.Path
	}

	httpReq, err := http.NewRequest(method, path, strings.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	// Copy headers (v2 sends single string per header, multi-values are comma-joined)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Copy query parameters
	q := httpReq.URL.Query()
	for key, value := range req.QueryStringParameters {
		q.Set(key, value)
	}
	httpReq.URL.RawQuery = q.Encode()

	// API Gateway v2 may put cookies in a separate Cookies field instead of
	// the headers map. Reconstruct the Cookie header from whichever source has them.
	if cookieHeader := req.Headers["cookie"]; cookieHeader != "" {
		httpReq.Header.Set("Cookie", cookieHeader)
	} else if len(req.Cookies) > 0 {
		httpReq.Header.Set("Cookie", strings.Join(req.Cookies, "; "))
	}

	return httpReq, nil
}

// responseWriter implements http.ResponseWriter for Lambda
// IMPORTANT: Header() returns a reference to the persistent headers map
// so that w.Header().Set(...) and http.SetCookie() work correctly.
type responseWriter struct {
	statusCode    int
	headers       http.Header
	body          *bytes.Buffer
	wroteHeader   bool
}

func newResponseWriter() *responseWriter {
	return &responseWriter{
		statusCode: http.StatusOK,
		headers:    make(http.Header),
		body:       &bytes.Buffer{},
	}
}

func (rw *responseWriter) Header() http.Header {
	return rw.headers
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	return rw.body.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if !rw.wroteHeader {
		rw.statusCode = statusCode
		rw.wroteHeader = true
	}
}

func main() {
	// Check if running in Lambda
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(Handler)
	} else {
		// Local development mode
		log.Println("Starting StreamMaxing API on :8080")

		// Load .env file for local development
		loadEnvFile()

		// Initialize database
		if err := db.Connect(); err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Println("Running without database connection - some features will not work")
		} else {
			defer db.Close()
			log.Println("Connected to database")
		}

		router := NewRouter()
		setupRoutes(router)

		handler := middleware.CORSMiddleware(middleware.LoggingMiddleware(router.ServeHTTP))

		log.Println("API server listening on http://localhost:8080")
		if err := http.ListenAndServe(":8080", http.HandlerFunc(handler)); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}
}

// loadEnvFile loads environment variables from .env file for local development
func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		log.Println("No .env file found - using system environment variables")
		return // .env file is optional
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Don't override existing env vars
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	log.Println("Loaded environment from .env")
}
