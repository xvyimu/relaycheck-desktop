package core

import (
	"context"
	"testing"
)

func TestValidateOutboundHTTPURLRejectsPrivateAndMetadataTargets(t *testing.T) {
	blocked := []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://10.0.0.1",
		"http://172.16.0.1",
		"http://192.168.1.1",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]:3000",
		"file:///etc/passwd",
	}
	for _, raw := range blocked {
		if _, err := validateOutboundHTTPURL(context.Background(), raw, outboundURLPolicy{}); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestValidateOutboundHTTPURLAllowsLoopbackOnlyWhenExplicit(t *testing.T) {
	if _, err := validateOutboundHTTPURL(context.Background(), "http://127.0.0.1:3000", outboundURLPolicy{}); err == nil {
		t.Fatal("expected loopback to be rejected by default")
	}
	if _, err := validateOutboundHTTPURL(context.Background(), "http://127.0.0.1:3000", outboundURLPolicy{AllowLocal: true}); err != nil {
		t.Fatalf("expected loopback to be allowed for local scan policy: %v", err)
	}
}

func TestValidateOutboundHTTPURLAllowsPublicIP(t *testing.T) {
	if _, err := validateOutboundHTTPURL(context.Background(), "https://93.184.216.34", outboundURLPolicy{}); err != nil {
		t.Fatalf("expected public IP to be allowed: %v", err)
	}
}
