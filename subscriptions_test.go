package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig(t *testing.T) config {
	t.Helper()
	var cfg config
	if err := json.Unmarshal(exampleConfig, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Token = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32))
	for i := range cfg.Nodes {
		cfg.Nodes[i].Password = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xff, 0xfe}, 8))
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSubscriptionSelection(t *testing.T) {
	cfg := testConfig(t)
	handler := subscriptionHandler(cfg)

	t.Run("Mihomo one device and all devices", func(t *testing.T) {
		cases := []struct {
			id        string
			wantNames []string
			omitted   []string
		}{
			{"windows", []string{"Windows"}, []string{"Android", "Linux"}},
			{"android", []string{"Android"}, []string{"Windows", "Linux"}},
			{"linux", []string{"Linux"}, []string{"Windows", "Android"}},
			{"all", []string{"Windows", "Android", "Linux"}, nil},
		}
		for _, test := range cases {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/sub/"+cfg.Token+"/"+test.id+"/mihomo.yaml", nil))
			if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("%s: status %d, headers %v", test.id, w.Code, w.Header())
			}
			body := w.Body.String()
			for _, name := range test.wantNames {
				if !strings.Contains(body, `name: "`+name+`"`) || !strings.Contains(body, `      - "`+name+`"`) {
					t.Fatalf("%s: missing usable node %s", test.id, name)
				}
			}
			for _, other := range test.omitted {
				if strings.Contains(body, other) {
					t.Fatalf("%s: unexpectedly includes %s", test.id, other)
				}
			}
			if !strings.Contains(body, `"MATCH,XiaYu Route"`) || strings.Contains(body, "DIRECT") {
				t.Fatalf("%s: expected proxy-only rule", test.id)
			}
		}
	})

	t.Run("SIP008 keeps UUID and endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/sub/"+cfg.Token+"/android/sip008.json", nil))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("status %d, headers %v", w.Code, w.Header())
		}
		var got struct {
			Version int            `json:"version"`
			Servers []sip008Server `json:"servers"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		entry := cfg.Nodes[1]
		want := sip008Server{entry.UUID, entry.Name, entry.Server, entry.Port, entry.Method, entry.Password}
		if got.Version != 1 || len(got.Servers) != 1 || got.Servers[0] != want {
			t.Fatal("SIP008 document does not contain the requested node")
		}
	})

	t.Run("bad token and unknown resource", func(t *testing.T) {
		for _, path := range []string{
			"/sub/wrong/android/sip008.json",
			"/sub/" + cfg.Token + "/unknown/sip008.json",
			"/sub/" + cfg.Token + "/android/unknown",
		} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), cfg.Nodes[0].Password) {
				t.Fatal("invalid request was not rejected")
			}
		}
	})
}

func TestShareLinkRoundTrip(t *testing.T) {
	entry := testConfig(t).Nodes[0]
	entry.Server = "2001:db8::42"
	entry.Name = "Windows #1"
	var buffer bytes.Buffer
	writeShareLinks(&buffer, []node{entry})
	raw := strings.TrimSpace(buffer.String())
	if !strings.Contains(raw, "%2B") || !strings.Contains(raw, "%2F") || !strings.Contains(raw, "%3D") {
		t.Fatal("SIP002 password must be percent-encoded")
	}
	link, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	password, ok := link.User.Password()
	if !ok || password != entry.Password || link.User.Username() != entry.Method ||
		link.Hostname() != entry.Server || link.Port() != "20001" || link.Fragment != entry.Name {
		t.Fatal("SIP002 link did not round-trip")
	}
}

func TestPrivateConfigInitialization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := initializeConfig(path); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config
	if err := json.Unmarshal(first, &cfg); err != nil {
		t.Fatal(err)
	}
	token, err := base64.RawURLEncoding.DecodeString(cfg.Token)
	ids := make(map[string]bool)
	for _, entry := range cfg.Nodes {
		ids[entry.UUID] = uuidPattern.MatchString(entry.UUID)
	}
	if err != nil || len(token) != 32 || len(cfg.Nodes) != 3 || len(ids) != 3 {
		t.Fatal("invalid token or node IDs in new configuration")
	}
	for _, valid := range ids {
		if !valid {
			t.Fatal("invalid UUID in new configuration")
		}
	}
	if err := initializeConfig(path); err == nil {
		t.Fatal("initializing an existing configuration must fail")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(first, after) {
		t.Fatal("initialization changed an existing configuration")
	}
}

func TestMihomoConfig(t *testing.T) {
	executable := os.Getenv("XIAYU_TEST_MIHOMO")
	if executable == "" {
		t.Skip("set XIAYU_TEST_MIHOMO to test with a Mihomo binary")
	}
	dir := t.TempDir()
	var fixture bytes.Buffer
	writeMihomo(&fixture, testConfig(t).Nodes)
	path := filepath.Join(dir, "subscription.yaml")
	if err := os.WriteFile(path, fixture.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(executable, "-t", "-d", dir, "-f", path).CombinedOutput(); err != nil {
		t.Fatalf("Mihomo rejected the YAML: %v\n%s", err, output)
	}
}
