package models

import "time"

type OrderStatus string

const (
	StatusNew        OrderStatus = "NEW"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusInvalid    OrderStatus = "INVALID"
	StatusProcessed  OrderStatus = "PROCESSED"
)

func (s OrderStatus) Final() bool {
	return s == StatusInvalid || s == StatusProcessed
}

type AccrualStatus string

const (
	AccrualRegistered AccrualStatus = "REGISTERED"
	AccrualInvalid    AccrualStatus = "INVALID"
	AccrualProcessing AccrualStatus = "PROCESSING"
	AccrualProcessed  AccrualStatus = "PROCESSED"
)

func (s AccrualStatus) Final() bool {
	return s == AccrualInvalid || s == AccrualProcessed
}

type RewardType string

const (
	RewardPercent RewardType = "%"
	RewardPoints  RewardType = "pt"
)

func (t RewardType) Valid() bool {
	return t == RewardPercent || t == RewardPoints
}

type UserOrder struct {
	Number     string      `json:"number"`
	Status     OrderStatus `json:"status"`
	Accrual    *float64    `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"uploaded_at"`
}

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdrawal struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type AccrualInfo struct {
	Order   string        `json:"order"`
	Status  AccrualStatus `json:"status"`
	Accrual *float64      `json:"accrual,omitempty"`
}

type OrderItem struct {
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

type GoodsRule struct {
	Match      string     `json:"match"`
	Reward     float64    `json:"reward"`
	RewardType RewardType `json:"reward_type"`
}
