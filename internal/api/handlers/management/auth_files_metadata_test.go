package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestPatchAuthFileMetadata_PersistsUserAgentAndListsIt(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fileName := "antigravity-user.json"
	filePath := filepath.Join(authDir, fileName)
	if errWrite := os.WriteFile(filePath, []byte(`{"type":"antigravity","email":"user@example.com","proxy_url":"http://old-proxy"}`), 0o600); errWrite != nil {
		t.Fatalf("failed to write auth file: %v", errWrite)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	body := `{"name":"antigravity-user.json","proxy_url":"","prefix":"","user_agent":"antigravity/9.9.9 linux/amd64"}`
	patchRec := httptest.NewRecorder()
	patchCtx, _ := gin.CreateTestContext(patchRec)
	patchReq := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/metadata", strings.NewReader(body))
	patchReq.Header.Set("Content-Type", "application/json")
	patchCtx.Request = patchReq
	h.PatchAuthFileMetadata(patchCtx)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected patch status %d, got %d with body %s", http.StatusOK, patchRec.Code, patchRec.Body.String())
	}

	savedData, errRead := os.ReadFile(filePath)
	if errRead != nil {
		t.Fatalf("failed to read saved auth file: %v", errRead)
	}

	var saved map[string]any
	if errUnmarshal := json.Unmarshal(savedData, &saved); errUnmarshal != nil {
		t.Fatalf("failed to decode saved auth file: %v", errUnmarshal)
	}
	if got := strings.TrimSpace(asString(saved["user_agent"])); got != "antigravity/9.9.9 linux/amd64" {
		t.Fatalf("expected user_agent to be persisted, got %#v", saved["user_agent"])
	}
	if _, exists := saved["proxy_url"]; exists {
		t.Fatalf("expected empty proxy_url to be removed from saved auth file, got %#v", saved["proxy_url"])
	}

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listReq := httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	listCtx.Request = listReq
	h.ListAuthFiles(listCtx)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	files := decodeListFiles(t, listRec.Body.Bytes())
	if len(files) != 1 {
		t.Fatalf("expected 1 auth file, got %d", len(files))
	}
	if got := strings.TrimSpace(asString(files[0]["user_agent"])); got != "antigravity/9.9.9 linux/amd64" {
		t.Fatalf("expected list response to expose user_agent, got %#v", files[0]["user_agent"])
	}
}

func TestListAuthFilesFromDisk_IncludesUserAgent(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fileName := "antigravity-disk.json"
	filePath := filepath.Join(authDir, fileName)
	if errWrite := os.WriteFile(filePath, []byte(`{"type":"antigravity","email":"disk@example.com","user_agent":"antigravity/1.2.3 darwin/arm64"}`), 0o600); errWrite != nil {
		t.Fatalf("failed to write auth file: %v", errWrite)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, nil)

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listReq := httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	listCtx.Request = listReq
	h.ListAuthFiles(listCtx)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	files := decodeListFiles(t, listRec.Body.Bytes())
	if len(files) != 1 {
		t.Fatalf("expected 1 auth file, got %d", len(files))
	}
	if got := strings.TrimSpace(asString(files[0]["user_agent"])); got != "antigravity/1.2.3 darwin/arm64" {
		t.Fatalf("expected disk list response to expose user_agent, got %#v", files[0]["user_agent"])
	}
}

func decodeListFiles(t *testing.T, body []byte) []map[string]any {
	t.Helper()

	var payload map[string]any
	if errUnmarshal := json.Unmarshal(body, &payload); errUnmarshal != nil {
		t.Fatalf("failed to decode list payload: %v", errUnmarshal)
	}

	filesRaw, ok := payload["files"].([]any)
	if !ok {
		t.Fatalf("expected files array, payload: %#v", payload)
	}

	files := make([]map[string]any, 0, len(filesRaw))
	for _, item := range filesRaw {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected file entry object, got %#v", item)
		}
		files = append(files, entry)
	}

	return files
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func TestListAuthFiles_WithManagerRegistrationIncludesUserAgent(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fileName := "antigravity-memory.json"
	filePath := filepath.Join(authDir, fileName)
	data := []byte(`{"type":"antigravity","email":"memory@example.com","user_agent":"antigravity/2.0.0 windows/amd64"}`)
	if errWrite := os.WriteFile(filePath, data, 0o600); errWrite != nil {
		t.Fatalf("failed to write auth file: %v", errWrite)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	if errRegister := h.registerAuthFromFile(context.Background(), filePath, data); errRegister != nil {
		t.Fatalf("failed to register auth file: %v", errRegister)
	}

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listReq := httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	listCtx.Request = listReq
	h.ListAuthFiles(listCtx)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	files := decodeListFiles(t, listRec.Body.Bytes())
	if len(files) != 1 {
		t.Fatalf("expected 1 auth file, got %d", len(files))
	}
	if got := strings.TrimSpace(asString(files[0]["user_agent"])); got != "antigravity/2.0.0 windows/amd64" {
		t.Fatalf("expected manager-backed list response to expose user_agent, got %#v", files[0]["user_agent"])
	}
}
