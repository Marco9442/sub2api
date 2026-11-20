//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestProviderClientIPKeepsOnlyLiteralAddress(t *testing.T) {
	marker := buildProviderCreatePaymentRequest(CreateOrderRequest{ClientIP: "proxy:baima-mesh"}, nil, "order", "1.00", "subject")
	require.Empty(t, marker.ClientIP)

	verified := buildProviderCreatePaymentRequest(CreateOrderRequest{ClientIP: "203.0.113.10"}, &payment.InstanceSelection{}, "order", "1.00", "subject")
	require.Equal(t, "203.0.113.10", verified.ClientIP)
}
