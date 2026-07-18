// Package api provides HTTP handlers for the PubSub server REST API.
package api

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/coregx/fursy"
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
	TopicCode  string          `json:"topicCode" validate:"required"`
	Identifier string          `json:"identifier"`
	Data       json.RawMessage `json:"data"`
}

// SubscribeRequest represents a subscription creation request.
type SubscribeRequest struct {
	SubscriberID int64  `json:"subscriberID" validate:"required,gt=0"`
	TopicCode    string `json:"topicCode" validate:"required"`
	Identifier   string `json:"identifier"`
}

// PublishResponse represents a successful publish response.
type PublishResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// SubscriptionResponse represents a successful subscription response.
type SubscriptionResponse struct {
	Success bool                `json:"success"`
	Data    *model.Subscription `json:"data"`
	Message string              `json:"message,omitempty"`
}

// SubscriptionListResponse represents a list of subscriptions.
type SubscriptionListResponse struct {
	Success bool                 `json:"success"`
	Data    []model.Subscription `json:"data"`
	Message string               `json:"message,omitempty"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// RegisterRoutes registers all API routes on the Fursy router.
func (h *Handler) RegisterRoutes(router *fursy.Router) {
	fursy.POST[PublishRequest, PublishResponse](router, "/api/v1/publish", h.handlePublish)
	fursy.POST[SubscribeRequest, SubscriptionResponse](router, "/api/v1/subscribe", h.handleSubscribe)
	fursy.GET[fursy.Empty, SubscriptionListResponse](router, "/api/v1/subscriptions", h.handleListSubscriptions)
	fursy.DELETE[fursy.Empty, PublishResponse](router, "/api/v1/subscriptions/:id", h.handleUnsubscribe)
	fursy.GET[fursy.Empty, HealthResponse](router, "/api/v1/health", h.handleHealth)
}

// handlePublish handles POST /api/v1/publish
func (h *Handler) handlePublish(box *fursy.Box[PublishRequest, PublishResponse]) error {
	result, err := h.publisher.Publish(box.Request.Context(), pubsub.PublishRequest{
		TopicCode:  box.ReqBody.TopicCode,
		Identifier: box.ReqBody.Identifier,
		Data:       string(box.ReqBody.Data),
	})
	if err != nil {
		h.logger.Errorf("Failed to publish message: %v", err)
		return box.Problem(fursy.InternalServerError("Failed to publish message"))
	}

	return box.Created("/api/v1/messages", PublishResponse{
		Success: true,
		Data:    result,
		Message: "Message published successfully",
	})
}

// handleSubscribe handles POST /api/v1/subscribe
func (h *Handler) handleSubscribe(box *fursy.Box[SubscribeRequest, SubscriptionResponse]) error {
	subscription, err := h.subscriptionManager.Subscribe(box.Request.Context(), pubsub.SubscribeRequest{
		SubscriberID: box.ReqBody.SubscriberID,
		TopicCode:    box.ReqBody.TopicCode,
		Identifier:   box.ReqBody.Identifier,
	})
	if err != nil {
		h.logger.Errorf("Failed to create subscription: %v", err)
		return box.Problem(fursy.InternalServerError("Failed to create subscription"))
	}

	return box.Created("/api/v1/subscriptions", SubscriptionResponse{
		Success: true,
		Data:    subscription,
		Message: "Subscription created successfully",
	})
}

// handleListSubscriptions handles GET /api/v1/subscriptions
func (h *Handler) handleListSubscriptions(box *fursy.Box[fursy.Empty, SubscriptionListResponse]) error {
	subscriberID, _ := strconv.ParseInt(box.Query("subscriberID"), 10, 64)
	identifier := box.Query("identifier")

	subscriptions, err := h.subscriptionManager.ListSubscriptions(box.Request.Context(), subscriberID, identifier)
	if err != nil {
		if pubsub.IsNoData(err) {
			return box.OK(SubscriptionListResponse{
				Success: true,
				Data:    []model.Subscription{},
				Message: "No subscriptions found",
			})
		}
		h.logger.Errorf("Failed to list subscriptions: %v", err)
		return box.Problem(fursy.InternalServerError("Failed to list subscriptions"))
	}

	return box.OK(SubscriptionListResponse{
		Success: true,
		Data:    subscriptions,
	})
}

// handleUnsubscribe handles DELETE /api/v1/subscriptions/:id
func (h *Handler) handleUnsubscribe(box *fursy.Box[fursy.Empty, PublishResponse]) error {
	subscriptionID, err := strconv.ParseInt(box.Param("id"), 10, 64)
	if err != nil {
		return box.Problem(fursy.BadRequest("Invalid subscription ID"))
	}

	subscription, err := h.subscriptionManager.Unsubscribe(box.Request.Context(), subscriptionID)
	if err != nil {
		if pubsub.IsNoData(err) {
			return box.Problem(fursy.NotFound("Subscription not found"))
		}
		h.logger.Errorf("Failed to unsubscribe: %v", err)
		return box.Problem(fursy.InternalServerError("Failed to unsubscribe"))
	}

	return box.OK(PublishResponse{
		Success: true,
		Data:    subscription,
		Message: "Unsubscribed successfully",
	})
}

// handleHealth handles GET /api/v1/health
func (h *Handler) handleHealth(box *fursy.Box[fursy.Empty, HealthResponse]) error {
	return box.OK(HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "0.2.0",
	})
}
