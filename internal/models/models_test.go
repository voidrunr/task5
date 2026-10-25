package models

import "testing"

func TestOrderStatusFinal(t *testing.T) {
	final := map[OrderStatus]bool{
		StatusNew:        false,
		StatusProcessing: false,
		StatusInvalid:    true,
		StatusProcessed:  true,
	}
	for status, want := range final {
		if got := status.Final(); got != want {
			t.Errorf("%s.Final() = %v, want %v", status, got, want)
		}
	}
}

func TestAccrualStatusFinal(t *testing.T) {
	final := map[AccrualStatus]bool{
		AccrualRegistered: false,
		AccrualProcessing: false,
		AccrualInvalid:    true,
		AccrualProcessed:  true,
	}
	for status, want := range final {
		if got := status.Final(); got != want {
			t.Errorf("%s.Final() = %v, want %v", status, got, want)
		}
	}
}

func TestRewardTypeValid(t *testing.T) {
	if !RewardPercent.Valid() {
		t.Error("RewardPercent should be valid")
	}
	if !RewardPoints.Valid() {
		t.Error("RewardPoints should be valid")
	}
	if RewardType("x").Valid() {
		t.Error(`RewardType("x") should be invalid`)
	}
}
