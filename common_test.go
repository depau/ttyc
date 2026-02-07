package ttyc

import (
	"net/http"
	"testing"
)

func TestGetHttpClient_Secure(t *testing.T) {
	client := GetHttpClient(false)
	if client == nil {
		t.Fatal("GetHttpClient returned nil")
	}
	
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	
	// When insecure is false, TLSClientConfig might be nil (default behavior)
	// or InsecureSkipVerify should be false
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be false when insecure=false")
	}
}

func TestGetHttpClient_Insecure(t *testing.T) {
	client := GetHttpClient(true)
	if client == nil {
		t.Fatal("GetHttpClient returned nil")
	}
	
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil when insecure=true")
	}
	
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true when insecure=true")
	}
}
