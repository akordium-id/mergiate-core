package sdk

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Money is an immutable Value Object representing a monetary amount in its minor unit (e.g. cents, Rupiah)
// paired with an ISO-4217 three-letter currency code.
type Money struct {
	amount   int64  // stored in minor currency unit (e.g., 10000 IDR = 10000, 10.50 USD = 1050)
	currency string // ISO-4217 code (e.g., "IDR", "USD")
}

// NewMoney creates a new Money value object.
func NewMoney(amount int64, currency string) (Money, error) {
	curr := strings.TrimSpace(strings.ToUpper(currency))
	if len(curr) != 3 {
		return Money{}, fmt.Errorf("%w: currency must be a 3-letter ISO code", ErrInvalidCurrency)
	}
	return Money{
		amount:   amount,
		currency: curr,
	}, nil
}

// MustNewMoney creates Money or panics if arguments are invalid.
func MustNewMoney(amount int64, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// ZeroMoney creates a zero-amount Money in the specified currency.
func ZeroMoney(currency string) Money {
	return MustNewMoney(0, currency)
}

// Amount returns the numerical value in minor currency units.
func (m Money) Amount() int64 {
	return m.amount
}

// Currency returns the ISO-4217 currency code.
func (m Money) Currency() string {
	return m.currency
}

// IsZero checks if the amount is zero.
func (m Money) IsZero() bool {
	return m.amount == 0
}

// IsPositive checks if the amount is strictly positive.
func (m Money) IsPositive() bool {
	return m.amount > 0
}

// IsNegative checks if the amount is strictly negative.
func (m Money) IsNegative() bool {
	return m.amount < 0
}

// Equals checks value equality between two Money objects.
func (m Money) Equals(other Money) bool {
	return m.amount == other.amount && m.currency == other.currency
}

// Add returns a new Money with the sum of both amounts.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: cannot add %s and %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{
		amount:   m.amount + other.amount,
		currency: m.currency,
	}, nil
}

// Subtract returns a new Money with the difference of both amounts.
func (m Money) Subtract(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: cannot subtract %s and %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{
		amount:   m.amount - other.amount,
		currency: m.currency,
	}, nil
}

// Multiply multiplies the money amount by an integer factor.
func (m Money) Multiply(factor int64) Money {
	return Money{
		amount:   m.amount * factor,
		currency: m.currency,
	}
}

// MultiplyFloat multiplies the money amount by a float factor, rounding to the nearest minor unit.
func (m Money) MultiplyFloat(factor float64) Money {
	val := float64(m.amount) * factor
	var rounded int64
	if val >= 0 {
		rounded = int64(val + 0.5)
	} else {
		rounded = int64(val - 0.5)
	}
	return Money{
		amount:   rounded,
		currency: m.currency,
	}
}

// Divide divides the money amount by an integer divisor.
func (m Money) Divide(divisor int64) (Money, error) {
	if divisor == 0 {
		return Money{}, ErrDivisionByZero
	}
	return Money{
		amount:   m.amount / divisor,
		currency: m.currency,
	}, nil
}

// String returns a formatted representation (e.g. "IDR 50000").
func (m Money) String() string {
	return fmt.Sprintf("%s %d", m.currency, m.amount)
}

// MarshalJSON implements json.Marshaler.
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"amount":   m.amount,
		"currency": m.currency,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Money) UnmarshalJSON(data []byte) error {
	var payload struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	parsed, err := NewMoney(payload.Amount, payload.Currency)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
