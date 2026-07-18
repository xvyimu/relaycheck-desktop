package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountResponsesDoNotSerializeBrowserProfilePaths(t *testing.T) {
	secretPath := `C:\Users\secret-user\relaycheck\data\browser-profiles\account-1`
	values := []interface{}{
		ChannelAccount{ID: "account-1", BrowserProfilePath: secretPath},
		browserLoginOpenResult{AccountID: "account-1", Status: "opened", ProfilePath: secretPath},
	}

	for _, value := range values {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "browserProfilePath") || strings.Contains(string(body), "profilePath") || strings.Contains(string(body), "secret-user") {
			t.Fatalf("public response serialized a browser profile path: %s", body)
		}
	}
}
