package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		wantBaseURL string
	}{
		{
			name:        "default proxy",
			baseURL:     "",
			wantBaseURL: "https://proxy.golang.org",
		},
		{
			name:        "custom proxy",
			baseURL:     "https://custom.proxy.com",
			wantBaseURL: "https://custom.proxy.com",
		},
		{
			name:        "custom proxy with trailing slash",
			baseURL:     "https://custom.proxy.com/",
			wantBaseURL: "https://custom.proxy.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL)

			if client == nil {
				t.Fatal("NewClient() returned nil")
			}

			if client.baseURL != tt.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.wantBaseURL)
			}

			if client.http == nil {
				t.Error("http client is nil")
			}

			if client.cache == nil {
				t.Error("cache is nil")
			}

			if client.http.Timeout != 30*time.Second {
				t.Errorf("http timeout = %v, want %v", client.http.Timeout, 30*time.Second)
			}
		})
	}
}

func TestClient_WithCache(t *testing.T) {
	client := NewClient("")
	mockCache := NewMemoryCache()

	result := client.WithCache(mockCache)

	if result != client {
		t.Error("WithCache() should return the same client instance")
	}

	if client.cache != mockCache {
		t.Error("WithCache() did not set the cache")
	}
}

func TestClient_Latest(t *testing.T) {
	expectedInfo := VersionInfo{
		Version: "v1.2.3",
		Time:    time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/@latest") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedInfo)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	info, err := client.Latest(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("Latest() error: %v", err)
	}

	if info == nil {
		t.Fatal("Latest() returned nil info")
	}

	if info.Version != expectedInfo.Version {
		t.Errorf("Version = %q, want %q", info.Version, expectedInfo.Version)
	}
}

func TestClient_Latest_Cached(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VersionInfo{
			Version: "v1.0.0",
			Time:    time.Now(),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Latest(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("First Latest() error: %v", err)
	}

	_, err = client.Latest(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("Second Latest() error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Server called %d times, want 1 (second call should use cache)", callCount)
	}
}

func TestClient_Latest_Error_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found: module not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Latest(ctx, "github.com/nonexistent/module")
	if err == nil {
		t.Fatal("Latest() should return error for 404")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention status code 404, got: %v", err)
	}
}

func TestClient_Latest_Error_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Latest(ctx, "github.com/test/module")
	if err == nil {
		t.Fatal("Latest() should return error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decoding") {
		t.Errorf("Error should mention decoding, got: %v", err)
	}
}

func TestClient_Latest_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(VersionInfo{Version: "v1.0.0"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Latest(ctx, "github.com/test/module")
	if err == nil {
		t.Fatal("Latest() should return error when context is canceled")
	}
}

func TestClient_Versions(t *testing.T) {
	expectedVersions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/@v/list") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Join(expectedVersions, "\n")))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	versions, err := client.Versions(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	if len(versions) != len(expectedVersions) {
		t.Fatalf("len(versions) = %d, want %d", len(versions), len(expectedVersions))
	}

	for i, v := range versions {
		if v != expectedVersions[i] {
			t.Errorf("versions[%d] = %q, want %q", i, v, expectedVersions[i])
		}
	}
}

func TestClient_Versions_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(""))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	versions, err := client.Versions(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("Versions() error: %v", err)
	}

	if len(versions) != 1 || versions[0] != "" {
		t.Errorf("Empty versions list should return [\"\"], got %v", versions)
	}
}

func TestClient_Versions_Cached(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("v1.0.0\nv1.1.0"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Versions(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("First Versions() error: %v", err)
	}

	_, err = client.Versions(ctx, "github.com/test/module")
	if err != nil {
		t.Fatalf("Second Versions() error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Server called %d times, want 1", callCount)
	}
}

func TestClient_Versions_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Versions(ctx, "github.com/test/module")
	if err == nil {
		t.Fatal("Versions() should return error for 500")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Error should mention status code 500, got: %v", err)
	}
}

func TestClient_Info(t *testing.T) {
	expectedInfo := VersionInfo{
		Version: "v1.2.3",
		Time:    time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ".info") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedInfo)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	info, err := client.Info(ctx, "github.com/test/module", "v1.2.3")
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}

	if info == nil {
		t.Fatal("Info() returned nil")
	}

	if info.Version != expectedInfo.Version {
		t.Errorf("Version = %q, want %q", info.Version, expectedInfo.Version)
	}

	if !info.Time.Equal(expectedInfo.Time) {
		t.Errorf("Time = %v, want %v", info.Time, expectedInfo.Time)
	}
}

func TestClient_Info_Cached(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(VersionInfo{Version: "v1.0.0", Time: time.Now()})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Info(ctx, "github.com/test/module", "v1.0.0")
	if err != nil {
		t.Fatalf("First Info() error: %v", err)
	}

	_, err = client.Info(ctx, "github.com/test/module", "v1.0.0")
	if err != nil {
		t.Fatalf("Second Info() error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Server called %d times, want 1", callCount)
	}
}

func TestClient_Info_DifferentVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := "v1.0.0"
		if strings.Contains(r.URL.Path, "v2.0.0") {
			version = "v2.0.0"
		}

		json.NewEncoder(w).Encode(VersionInfo{Version: version, Time: time.Now()})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	info1, err := client.Info(ctx, "github.com/test/module", "v1.0.0")
	if err != nil {
		t.Fatalf("Info(v1.0.0) error: %v", err)
	}

	info2, err := client.Info(ctx, "github.com/test/module", "v2.0.0")
	if err != nil {
		t.Fatalf("Info(v2.0.0) error: %v", err)
	}

	if info1.Version == info2.Version {
		t.Error("Different versions should return different info")
	}
}

func TestClient_Info_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("version not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Info(ctx, "github.com/test/module", "v99.99.99")
	if err == nil {
		t.Fatal("Info() should return error for 404")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention 404, got: %v", err)
	}
}

func TestClient_GetModFile(t *testing.T) {
	expectedContent := []byte(`module github.com/test/module

go 1.24.2

require github.com/dep/module v1.0.0
`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".mod") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write(expectedContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	data, err := client.GetModFile(ctx, "github.com/test/module", "v1.0.0")
	if err != nil {
		t.Fatalf("GetModFile() error: %v", err)
	}

	if string(data) != string(expectedContent) {
		t.Errorf("GetModFile() = %q, want %q", string(data), string(expectedContent))
	}
}

func TestClient_GetModFile_Cached(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("module test"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetModFile(ctx, "github.com/test/module", "v1.0.0")
	if err != nil {
		t.Fatalf("First GetModFile() error: %v", err)
	}

	_, err = client.GetModFile(ctx, "github.com/test/module", "v1.0.0")
	if err != nil {
		t.Fatalf("Second GetModFile() error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Server called %d times, want 1", callCount)
	}
}

func TestClient_GetModFile_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("mod file not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetModFile(ctx, "github.com/test/module", "v1.0.0")
	if err == nil {
		t.Fatal("GetModFile() should return error for 404")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention 404, got: %v", err)
	}
}

func TestClient_HTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(VersionInfo{Version: "v1.0.0"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.http.Timeout = 10 * time.Millisecond

	ctx := context.Background()

	_, err := client.Latest(ctx, "github.com/test/module")
	if err == nil {
		t.Fatal("Latest() should timeout")
	}
}

func TestClient_CacheIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/@latest"):
			json.NewEncoder(w).Encode(VersionInfo{Version: "v2.0.0", Time: time.Now()})
		case strings.Contains(r.URL.Path, ".info"):
			json.NewEncoder(w).Encode(VersionInfo{Version: "v1.0.0", Time: time.Now()})
		case strings.Contains(r.URL.Path, ".mod"):
			w.Write([]byte("module test"))
		case strings.Contains(r.URL.Path, "/list"):
			w.Write([]byte("v1.0.0\nv2.0.0"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	latest, _ := client.Latest(ctx, "github.com/test/module")
	info, _ := client.Info(ctx, "github.com/test/module", "v1.0.0")
	versions, _ := client.Versions(ctx, "github.com/test/module")
	modFile, _ := client.GetModFile(ctx, "github.com/test/module", "v1.0.0")

	if latest.Version == info.Version {
		t.Error("Latest and Info should have different cached versions")
	}

	if len(versions) == 0 {
		t.Error("Versions should be cached")
	}

	if len(modFile) == 0 {
		t.Error("ModFile should be cached")
	}
}

func TestClient_ErrorHandling_ReadAllFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.Versions(ctx, "github.com/test/module")
	if err == nil {
		t.Fatal("Versions() should error on connection close")
	}
}

func TestVersionInfo_JSONMarshaling(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := VersionInfo{
		Version: "v1.2.3",
		Time:    now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded VersionInfo
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, original.Version)
	}

	if !decoded.Time.Equal(original.Time) {
		t.Errorf("Time = %v, want %v", decoded.Time, original.Time)
	}
}

// Benchmark tests

func BenchmarkClient_Latest(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(VersionInfo{Version: "v1.0.0", Time: time.Now()})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.cache = &noOpCache{}
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		client.Latest(ctx, "github.com/test/module")
	}
}

func BenchmarkClient_Latest_Cached(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(VersionInfo{Version: "v1.0.0", Time: time.Now()})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	client.Latest(ctx, "github.com/test/module")

	b.ResetTimer()
	for b.Loop() {
		client.Latest(ctx, "github.com/test/module")
	}
}

func BenchmarkClient_GetModFile(b *testing.B) {
	modContent := []byte(`module github.com/test/module

go 1.24.2
`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(modContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.cache = &noOpCache{}
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		client.GetModFile(ctx, "github.com/test/module", "v1.0.0")
	}
}

type noOpCache struct{}

func (n *noOpCache) Get(key string) (any, bool)                   { return nil, false }
func (n *noOpCache) Set(key string, value any, ttl time.Duration) {}
func (n *noOpCache) Clear()                                       {}
