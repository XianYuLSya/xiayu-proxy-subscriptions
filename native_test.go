package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestExportNativeNode(t *testing.T) {
	cfg := testConfig(t)
	cfg.Nodes[1].UDP = false
	dir := t.TempDir()
	for _, entry := range cfg.Nodes {
		t.Run(entry.ID, func(t *testing.T) {
			file := filepath.Join(dir, entry.ID+".server.json")
			if err := exportNodeConfig(cfg, entry.ID, "127.0.0.1", file); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(cfg.Token)) || bytes.Contains(data, []byte(entry.Server)) {
				t.Fatal("export includes subscription-only fields")
			}
			var got singBoxConfig
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Inbounds) != 1 || len(got.Outbounds) != 1 {
				t.Fatal("expected one inbound and one outbound")
			}
			inbound, outbound := got.Inbounds[0], got.Outbounds[0]
			if inbound.Type != "shadowsocks" || inbound.Port != entry.Port ||
				inbound.Listen != "127.0.0.1" || inbound.Method != entry.Method || inbound.Password != entry.Password {
				t.Fatal("unexpected inbound")
			}
			wantNetwork := ""
			if !entry.UDP {
				wantNetwork = "tcp"
			}
			if inbound.Network != wantNetwork || outbound.Type != "direct" || got.Route["final"] != outbound.Tag {
				t.Fatal("unexpected transport or egress")
			}
			if err := exportNodeConfig(cfg, entry.ID, "::", file); err == nil {
				t.Fatal("an existing configuration must only be changed explicitly")
			}
		})
	}

	file := filepath.Join(dir, "invalid.server.json")
	if err := exportNodeConfig(cfg, "missing", "127.0.0.1", file); err == nil {
		t.Fatal("unknown node must be rejected")
	}
	if err := exportNodeConfig(cfg, "linux", "127.0.0.1:8388", file); err == nil {
		t.Fatal("a bind address with a port must be rejected")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("unexpected output for invalid arguments")
	}
}

func TestSingBoxConfig(t *testing.T) {
	executable := os.Getenv("XIAYU_TEST_SINGBOX")
	if executable == "" {
		t.Skip("set XIAYU_TEST_SINGBOX to test with a sing-box binary")
	}
	cfg := testConfig(t)
	dir := t.TempDir()
	for _, entry := range cfg.Nodes {
		t.Run(entry.ID, func(t *testing.T) {
			file := filepath.Join(dir, entry.ID+".server.json")
			if err := exportNodeConfig(cfg, entry.ID, "127.0.0.1", file); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, executable, "check", "-c", file).CombinedOutput(); err != nil {
				t.Fatalf("sing-box rejected the configuration: %v\n%s", err, output)
			}
		})
	}
}

func TestSingBoxRelay(t *testing.T) {
	executable := os.Getenv("XIAYU_TEST_SINGBOX")
	if executable == "" {
		t.Skip("set XIAYU_TEST_SINGBOX to test with a sing-box binary")
	}
	dir := t.TempDir()
	edge := loopbackListener(t)
	client := loopbackListener(t)
	defer edge.Close()
	defer client.Close()
	edgePort := edge.Addr().(*net.TCPAddr).Port
	clientPort := client.Addr().(*net.TCPAddr).Port
	cfg := testConfig(t)
	cfg.Nodes[0].Port = edgePort
	entry := cfg.Nodes[0]
	edgeFile := filepath.Join(dir, "edge.server.json")
	if err := exportNodeConfig(cfg, entry.ID, "127.0.0.1", edgeFile); err != nil {
		t.Fatal(err)
	}
	clientFile := filepath.Join(dir, "client.json")
	clientConfig := map[string]any{
		"inbounds": []map[string]any{{
			"type": "mixed", "listen": "127.0.0.1", "listen_port": clientPort,
		}},
		"outbounds": []map[string]any{{
			"type": "shadowsocks", "server": "127.0.0.1", "server_port": edgePort,
			"method": entry.Method, "password": entry.Password,
		}},
	}
	if err := writePrivateJSON(clientFile, clientConfig); err != nil {
		t.Fatal(err)
	}

	edge.Close()
	startLocalSingBox(t, executable, edgeFile, edgePort)
	client.Close()
	startLocalSingBox(t, executable, clientFile, clientPort)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "received through Shadowsocks")
	}))
	defer target.Close()
	proxy := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(clientPort))}
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := httpClient.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "received through Shadowsocks" {
		t.Fatal("the generated server configuration did not relay the request")
	}
}

func loopbackListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func startLocalSingBox(t *testing.T, executable, file string, port int) {
	t.Helper()
	cmd := exec.Command(executable, "run", "-c", file)
	cmd.Dir = filepath.Dir(file)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp4", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("sing-box did not start listening on %s", addr)
}
