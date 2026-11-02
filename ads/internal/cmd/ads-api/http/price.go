package http

import (
	"encoding/json"
	"fmt"
)

func centsToPrice(cents int64) json.Number {
	euros := cents / 100
	remainder := cents % 100

	if remainder < 0 {
		remainder = -remainder
	}

	if cents < 0 && euros == 0 {
		return json.Number(fmt.Sprintf("-0.%02d", remainder))
	}

	return json.Number(fmt.Sprintf("%d.%02d", euros, remainder))
}
