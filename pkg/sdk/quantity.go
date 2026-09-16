package sdk

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Unit represents a Unit of Measure (UoM) definition.
type Unit struct {
	Code             string  `json:"code"`                        // e.g. "PCS", "KG", "M", "BOX"
	Name             string  `json:"name"`                        // e.g. "Piece", "Kilogram"
	Symbol           string  `json:"symbol"`                      // e.g. "pcs", "kg"
	BaseUnitCode     string  `json:"base_unit_code,omitempty"`    // base unit if derived (e.g. "G" for "KG")
	ConversionFactor float64 `json:"conversion_factor,omitempty"` // multiplier to convert to base unit
}

// Quantity is an immutable Value Object representing an amount accompanied by a Unit of Measure.
type Quantity struct {
	value    float64
	unitCode string
}

// NewQuantity creates a new Quantity.
func NewQuantity(value float64, unitCode string) (Quantity, error) {
	code := strings.TrimSpace(strings.ToUpper(unitCode))
	if code == "" {
		return Quantity{}, fmt.Errorf("%w: unit code cannot be empty", ErrInvalidInput)
	}
	return Quantity{
		value:    value,
		unitCode: code,
	}, nil
}

// MustNewQuantity creates Quantity or panics.
func MustNewQuantity(value float64, unitCode string) Quantity {
	q, err := NewQuantity(value, unitCode)
	if err != nil {
		panic(err)
	}
	return q
}

// Value returns the numeric quantity value.
func (q Quantity) Value() float64 {
	return q.value
}

// UnitCode returns the unit code.
func (q Quantity) UnitCode() string {
	return q.unitCode
}

// IsZero checks if quantity is zero.
func (q Quantity) IsZero() bool {
	return math.Abs(q.value) < 1e-9
}

// IsPositive checks if quantity is strictly positive.
func (q Quantity) IsPositive() bool {
	return q.value > 1e-9
}

// IsNegative checks if quantity is strictly negative.
func (q Quantity) IsNegative() bool {
	return q.value < -1e-9
}

// Equals checks value equality within float tolerance.
func (q Quantity) Equals(other Quantity) bool {
	if q.unitCode != other.unitCode {
		return false
	}
	return math.Abs(q.value-other.value) < 1e-9
}

// Add adds another quantity with identical unit.
func (q Quantity) Add(other Quantity) (Quantity, error) {
	if q.unitCode != other.unitCode {
		return Quantity{}, fmt.Errorf("%w: cannot add %s and %s", ErrUnitMismatch, q.unitCode, other.unitCode)
	}
	return Quantity{
		value:    q.value + other.value,
		unitCode: q.unitCode,
	}, nil
}

// Subtract subtracts another quantity with identical unit.
func (q Quantity) Subtract(other Quantity) (Quantity, error) {
	if q.unitCode != other.unitCode {
		return Quantity{}, fmt.Errorf("%w: cannot subtract %s and %s", ErrUnitMismatch, q.unitCode, other.unitCode)
	}
	return Quantity{
		value:    q.value - other.value,
		unitCode: q.unitCode,
	}, nil
}

// Multiply multiplies the quantity by a scalar factor.
func (q Quantity) Multiply(factor float64) Quantity {
	return Quantity{
		value:    q.value * factor,
		unitCode: q.unitCode,
	}
}

// Divide divides the quantity by a scalar divisor.
func (q Quantity) Divide(divisor float64) (Quantity, error) {
	if math.Abs(divisor) < 1e-9 {
		return Quantity{}, ErrDivisionByZero
	}
	return Quantity{
		value:    q.value / divisor,
		unitCode: q.unitCode,
	}, nil
}

// String returns a human readable representation (e.g. "15.50 KG").
func (q Quantity) String() string {
	return fmt.Sprintf("%.2f %s", q.value, q.unitCode)
}

// MarshalJSON implements json.Marshaler.
func (q Quantity) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"value":     q.value,
		"unit_code": q.unitCode,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (q *Quantity) UnmarshalJSON(data []byte) error {
	var payload struct {
		Value    float64 `json:"value"`
		UnitCode string  `json:"unit_code"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	parsed, err := NewQuantity(payload.Value, payload.UnitCode)
	if err != nil {
		return err
	}
	*q = parsed
	return nil
}
