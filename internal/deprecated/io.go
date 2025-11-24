package pubsub

import (
	"github.com/coregx/pubsub/model"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type SubscriptionResponse struct {
	model.Subscription
}

type CreateSubscriptionRequest struct {
	SubscriberID int64  `json:"clientID"`
	CbuID        int64  `json:"cbuId"`
	TopicCode    string `json:"class"`
	Identifier   string `json:"identifier"`
	CallbackUrl  string `json:"callbackURL"`
	TrackID      int64  `json:"trackId"`
}

func (m CreateSubscriptionRequest) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.SubscriberID, validation.Required),
		validation.Field(&m.CallbackUrl, validation.Required, validation.Length(3, 255)),
	)
}

type CreateSubscriptionResponse struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

type Filter struct {
	SubscriberID int
	CbuID        int
	TopicID      string
	IsActive     bool
}

type DeleteSubscriptionRequest struct {
	ClientID int64 `json:"clientID"`
	TrackID  int64 `json:"trackId"`
}

func (m DeleteSubscriptionRequest) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.ClientID, validation.Required),
		validation.Field(&m.TrackID, validation.Required),
	)
}

type DeleteSubscriptionResponse struct {
	model.Subscription
}

type ListSubscriptionsResponse struct {
	Items []model.Subscription `json:"items"`
}

type PublishMessage struct {
	model.DataMessage
}
