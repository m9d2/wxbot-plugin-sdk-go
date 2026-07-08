package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestDevHandlerProvidesHostAndMockAccounts(t *testing.T) {
	cfg := DevConfig{}
	applyDevDefaults(&cfg)
	handler, err := newDevHandler(devManifest{ID: "demo-plugin"}, cfg)
	if err != nil {
		t.Fatal(err)
	}

	hostResponse := httptest.NewRecorder()
	handler.ServeHTTP(hostResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(hostResponse.Body.String(), "wxbot-plugin:init") ||
		!strings.Contains(hostResponse.Body.String(), "wxbot-plugin-context") {
		t.Fatalf("development host does not inject both context protocols")
	}

	accountsResponse := httptest.NewRecorder()
	handler.ServeHTTP(accountsResponse, httptest.NewRequest(http.MethodGet, "/api/plugins/demo-plugin/sdk/accounts", nil))
	var payload struct {
		Success  bool         `json:"success"`
		Accounts []DevAccount `json:"accounts"`
	}
	if err := json.Unmarshal(accountsResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success || len(payload.Accounts) != 1 || payload.Accounts[0].Wxid != "wxid_dev" {
		t.Fatalf("unexpected accounts response: %+v", payload)
	}
}

func TestValidateDevConfigRejectsInvalidPort(t *testing.T) {
	cfg := DevConfig{}
	applyDevDefaults(&cfg)
	cfg.BackendPort = 70000
	if err := validateDevConfig(cfg); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestDevHandlerProxiesPluginAPIWithContext(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/settings" {
			t.Errorf("unexpected backend path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Wxbot-User-ID"); got != "7" {
			t.Errorf("unexpected user id: %s", got)
		}
		if got := r.Header.Get("X-Wxbot-Account-Wxid"); got != "wxid_selected" {
			t.Errorf("unexpected account wxid: %s", got)
		}
		if got := r.Header.Get("X-Wxbot-Plugin-ID"); got != "demo-plugin" {
			t.Errorf("unexpected plugin id: %s", got)
		}
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(backendURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	cfg := DevConfig{
		Host:        backendURL.Hostname(),
		BackendPort: port,
		UserID:      7,
		Accounts:    []DevAccount{{Wxid: "wxid_dev"}},
	}
	handler, err := newDevHandler(devManifest{ID: "demo-plugin"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/plugins/demo-plugin/api/settings",
		strings.NewReader(`{"wxid":"wxid_selected"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}
