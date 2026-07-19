package decision

import (
	"fmt"
	"math"

	"nofx/store"
)

// CapPositionSize is the common position-limit policy used by live execution
// and backtesting. Exchange-specific affordability and rounding remain in the
// adapter layer.
func CapPositionSize(requested, currentNotional float64, risk store.RiskControlConfig) float64 {
	size := math.Max(requested, 0)
	if risk.MaxPositionSize > 0 {
		size = math.Min(size, risk.MaxPositionSize)
	}
	if risk.MaxTotalPositionSize > 0 {
		size = math.Min(size, math.Max(risk.MaxTotalPositionSize-currentNotional, 0))
	}
	return size
}

func ValidatePositionCount(current int, risk store.RiskControlConfig) error {
	maxPositions := risk.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3
	}
	if current >= maxPositions {
		return fmt.Errorf("already at max positions (%d/%d)", current, maxPositions)
	}
	return nil
}

func ValidateMinimumPositionSize(size float64, risk store.RiskControlConfig) error {
	minimum := risk.MinPositionSize
	if minimum <= 0 {
		minimum = 12
	}
	if size < minimum {
		return fmt.Errorf("position %.2f USDT below minimum (%.2f USDT)", size, minimum)
	}
	return nil
}
