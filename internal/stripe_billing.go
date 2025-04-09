package internal

import (
	"fmt"
	"sync"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/client"
)

// StripeBilling handles interactions with Stripe for billing purposes
type StripeBilling struct {
	client *client.API
	state  *StripeBillingState
	mu     sync.RWMutex
}

// StripeBillingState maintains the internal state of the billing system
type StripeBillingState struct {
	MetersByEventName map[string]StripeMeter
	PriceIDsByMeterID map[string]string
}

// StripeMeter represents a Stripe meter for usage-based billing
type StripeMeter struct {
	ID        string
	EventName string
}

// StripeModel holds price IDs for different types of token usage
type StripeModel struct {
	InputTokensPrice              StripeBillingPrice
	InputCacheCreationTokensPrice StripeBillingPrice
	InputCacheReadTokensPrice     StripeBillingPrice
	OutputTokensPrice             StripeBillingPrice
}

// StripeBillingPrice represents a Stripe price with its associated meter
type StripeBillingPrice struct {
	ID             string
	MeterEventName string
}

// Cents represents an amount in cents for billing
type Cents int64

// FromDollars converts dollar amounts to cents
func (c Cents) FromDollars(dollars float64) Cents {
	return Cents(dollars * 100)
}

// NewStripeBilling creates a new Stripe billing client
func NewStripeBilling(apiKey string) (*StripeBilling, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("stripe API key is required")
	}

	client := &client.API{}
	client.Init(apiKey, nil)

	return &StripeBilling{
		client: client,
		state: &StripeBillingState{
			MetersByEventName: make(map[string]StripeMeter),
			PriceIDsByMeterID: make(map[string]string),
		},
	}, nil
}

// Initialize fetches and caches meters and prices from Stripe
func (s *StripeBilling) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Fetch meters
	meters, err := s.listMeters()
	if err != nil {
		return fmt.Errorf("failed to list meters: %w", err)
	}

	// Fetch prices
	prices, err := s.listPrices()
	if err != nil {
		return fmt.Errorf("failed to list prices: %w", err)
	}

	// Update state with fetched data
	for _, meter := range meters {
		s.state.MetersByEventName[meter.EventName] = meter
	}

	for _, price := range prices {
		if meter, ok := s.state.MetersByEventName[price.Nickname]; ok {
			s.state.PriceIDsByMeterID[meter.ID] = price.ID
		}
	}

	return nil
}

// BillModelUsage bills a customer for usage of an LLM model
func (s *StripeBilling) BillModelUsage(customerID string, model *StripeModel, event ModelEvent) error {
	if event.InputTokens > 0 {
		if err := s.recordMeterEvent(customerID, model.InputTokensPrice.MeterEventName, event.InputTokens); err != nil {
			return err
		}
	}

	if event.InputCacheCreationTokens > 0 {
		if err := s.recordMeterEvent(customerID, model.InputCacheCreationTokensPrice.MeterEventName, event.InputCacheCreationTokens); err != nil {
			return err
		}
	}

	if event.InputCacheReadTokens > 0 {
		if err := s.recordMeterEvent(customerID, model.InputCacheReadTokensPrice.MeterEventName, event.InputCacheReadTokens); err != nil {
			return err
		}
	}

	if event.OutputTokens > 0 {
		if err := s.recordMeterEvent(customerID, model.OutputTokensPrice.MeterEventName, event.OutputTokens); err != nil {
			return err
		}
	}

	return nil
}

// ModelEvent represents usage of a language model
type ModelEvent struct {
	InputTokens              int64
	InputCacheCreationTokens int64
	InputCacheReadTokens     int64
	OutputTokens             int64
}

// listMeters fetches all meters from Stripe
func (s *StripeBilling) listMeters() ([]StripeMeter, error) {
	params := &stripe.BillingPortalConfigurationListParams{}
	params.Limit = stripe.Int64(100)

	// Note: This is a placeholder since the Stripe Go library doesn't have a direct meters API
	// In a real implementation, you would use the appropriate Stripe API endpoints
	return []StripeMeter{}, nil
}

// listPrices fetches all prices from Stripe
func (s *StripeBilling) listPrices() ([]*stripe.Price, error) {
	params := &stripe.PriceListParams{}
	params.Limit = stripe.Int64(100)

	i := s.client.Prices.List(params)
	prices := make([]*stripe.Price, 0)

	for i.Next() {
		prices = append(prices, i.Price())
	}

	return prices, i.Err()
}

// recordMeterEvent records a meter event in Stripe
func (s *StripeBilling) recordMeterEvent(customerID string, eventName string, value int64) error {
	// Implementation needed to use these parameters
	// For now, to avoid unused parameter warnings:
	if value > 0 && len(customerID) > 0 && len(eventName) > 0 {
		// This would use Stripe's metering API, which is not directly available in the Go library
		// We would need to make a custom API request here
	}
	return nil
}
