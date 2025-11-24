// Package api provides HTTP handlers for the PubSub server REST API.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/coregx/pubsub"
	"github.com/coregx/pubsub/model"
)

// Handler holds dependencies for API handlers.
type Handler struct {
	publisher           *pubsub.Publisher
	subscriptionManager *pubsub.SubscriptionManager
	logger              pubsub.Logger
}

// NewHandler creates a new API handler.
func NewHandler(
	publisher *pubsub.Publisher,
	subscriptionManager *pubsub.SubscriptionManager,
	logger pubsub.Logger,
) *Handler {
	return &Handler{
		publisher:           publisher,
		subscriptionManager: subscriptionManager,
		logger:              logger,
	}
}

// PublishRequest represents a publish message request.
type PublishRequest struct {
	TopicCode  string                 `json:"topicCode"`
	Identifier string                 `json:"identifier"`
	Data       map[string]interface{} `json:"data"`
}

// SubscribeRequest represents a subscription creation request.
type SubscribeRequest struct {
	SubscriberID int64  `json:"subscriberID"`
	TopicCode    string `json:"topicCode"`
	Identifier   string `json:"identifier"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a success response.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// HandlePublish handles POST /api/v1/publish
func (h *Handler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
		return
	}

	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	// Validate request
	if req.TopicCode == "" {
		h.respondError(w, http.StatusBadRequest, "topicCode is required", "VALIDATION_ERROR")
		return
	}

	// Convert data to JSON string
	dataJSON, err := json.Marshal(req.Data)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "Failed to serialize data", "SERIALIZATION_ERROR")
		return
	}

	// Publish message
	result, err := h.publisher.Publish(r.Context(), pubsub.PublishRequest{
		TopicCode:  req.TopicCode,
		Identifier: req.Identifier,
		Data:       string(dataJSON),
	})

	if err != nil {
		h.logger.Errorf("Failed to publish message: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to publish message", "PUBLISH_ERROR")
		return
	}

	h.respondSuccess(w, http.StatusCreated, result, "Message published successfully")
}

// HandleSubscribe handles POST /api/v1/subscribe
func (h *Handler) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
		return
	}

	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid JSON", "INVALID_JSON")
		return
	}

	// Validate request
	if req.SubscriberID == 0 || req.TopicCode == "" {
		h.respondError(w, http.StatusBadRequest, "subscriberID and topicCode are required", "VALIDATION_ERROR")
		return
	}

	// Create subscription
	subscription, err := h.subscriptionManager.Subscribe(r.Context(), pubsub.SubscribeRequest{
		SubscriberID: req.SubscriberID,
		TopicCode:    req.TopicCode,
		Identifier:   req.Identifier,
	})

	if err != nil {
		h.logger.Errorf("Failed to create subscription: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to create subscription", "SUBSCRIBE_ERROR")
		return
	}

	h.respondSuccess(w, http.StatusCreated, subscription, "Subscription created successfully")
}

// HandleListSubscriptions handles GET /api/v1/subscriptions
func (h *Handler) HandleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
		return
	}

	// Parse query parameters
	subscriberID, _ := strconv.ParseInt(r.URL.Query().Get("subscriberID"), 10, 64)
	identifier := r.URL.Query().Get("identifier")

	// List subscriptions
	subscriptions, err := h.subscriptionManager.ListSubscriptions(r.Context(), subscriberID, identifier)
	if err != nil {
		if pubsub.IsNoData(err) {
			h.respondSuccess(w, http.StatusOK, []model.Subscription{}, "No subscriptions found")
			return
		}
		h.logger.Errorf("Failed to list subscriptions: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to list subscriptions", "LIST_ERROR")
		return
	}

	h.respondSuccess(w, http.StatusOK, subscriptions, "")
}

// HandleUnsubscribe handles DELETE /api/v1/subscriptions/:id
func (h *Handler) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
		return
	}

	// Extract subscription ID from path (simple parsing)
	// In production, use a router like gorilla/mux or chi
	pathParts := splitPath(r.URL.Path)
	if len(pathParts) < 4 {
		h.respondError(w, http.StatusBadRequest, "Invalid subscription ID", "INVALID_ID")
		return
	}

	subscriptionID, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid subscription ID", "INVALID_ID")
		return
	}

	// Unsubscribe
	subscription, err := h.subscriptionManager.Unsubscribe(r.Context(), subscriptionID)
	if err != nil {
		if pubsub.IsNoData(err) {
			h.respondError(w, http.StatusNotFound, "Subscription not found", "NOT_FOUND")
			return
		}
		h.logger.Errorf("Failed to unsubscribe: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to unsubscribe", "UNSUBSCRIBE_ERROR")
		return
	}

	h.respondSuccess(w, http.StatusOK, subscription, "Unsubscribed successfully")
}

// HandleHealth handles GET /api/v1/health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "0.1.0",
	}

	h.respondSuccess(w, http.StatusOK, health, "")
}

// respondError sends an error response.
func (h *Handler) respondError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Code:    code,
		Message: message,
	})
}

// respondSuccess sends a success response.
func (h *Handler) respondSuccess(w http.ResponseWriter, status int, data interface{}, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	})
}

// splitPath splits URL path by "/"
func splitPath(path string) []string {
	parts := []string{}
	for _, part := range splitString(path, '/') {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// splitString splits string by separator (simple implementation)
func splitString(s string, sep rune) []string {
	var parts []string
	var current string
	for _, c := range s {
		if c == sep {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}
