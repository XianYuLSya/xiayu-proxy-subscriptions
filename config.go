package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"unicode"
)

//go:embed config.example.json
var exampleConfig []byte

type config struct {
	Token string `json:"token"`
	Nodes []node `json:"nodes"`
}

type node struct {
	ID       string `json:"id"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Method   string `json:"method"`
	Password string `json:"password"`
	UDP      bool   `json:"udp"`
}

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hostPattern = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*\.?$`)
)

func loadConfig(path string) (config, error) {
	var cfg config
	file, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return cfg, errors.New("config must contain one JSON object")
	}
	return cfg, cfg.validate()
}

func (cfg config) validate() error {
	token, err := base64.RawURLEncoding.Strict().DecodeString(cfg.Token)
	if err != nil || len(token) != 32 || len(cfg.Token) != 43 {
		return errors.New("config token must contain 32 random bytes encoded as unpadded base64url; use -init to generate one")
	}
	if len(cfg.Nodes) == 0 {
		return errors.New("config must contain at least one node")
	}

	ids := make(map[string]bool)
	uuids := make(map[string]bool)
	names := map[string]bool{
		routeGroup: true, "DIRECT": true, "REJECT": true, "REJECT-DROP": true,
		"COMPATIBLE": true, "PASS": true, "GLOBAL": true,
	}
	for i, entry := range cfg.Nodes {
		if !idPattern.MatchString(entry.ID) || entry.ID == "all" || ids[entry.ID] {
			return fmt.Errorf("node %d: id must be unique, use lowercase letters, digits and hyphens, and differ from all", i+1)
		}
		uuid := strings.ToLower(entry.UUID)
		if !uuidPattern.MatchString(uuid) || uuids[uuid] {
			return fmt.Errorf("node %s: uuid must be a unique UUID", entry.ID)
		}
		if entry.Name == "" || strings.TrimSpace(entry.Name) != entry.Name ||
			strings.ContainsFunc(entry.Name, unicode.IsControl) || names[entry.Name] {
			return fmt.Errorf("node %s: name must be a unique, non-reserved display name", entry.ID)
		}
		if !validServer(entry.Server) || entry.Port < 1 || entry.Port > 65535 {
			return fmt.Errorf("node %s: provide a hostname or IP address and a valid port", entry.ID)
		}
		wantKeyLength := 0
		switch entry.Method {
		case "2022-blake3-aes-128-gcm":
			wantKeyLength = 16
		case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
			wantKeyLength = 32
		default:
			return fmt.Errorf("node %s: use a supported Shadowsocks AEAD-2022 method", entry.ID)
		}
		key, err := base64.StdEncoding.Strict().DecodeString(entry.Password)
		if err != nil || len(key) != wantKeyLength || base64.StdEncoding.EncodeToString(key) != entry.Password {
			return fmt.Errorf("node %s: password must be a standard Base64 key of %d bytes", entry.ID, wantKeyLength)
		}

		ids[entry.ID] = true
		uuids[uuid] = true
		names[entry.Name] = true
	}
	return nil
}

func validServer(server string) bool {
	if addr, err := netip.ParseAddr(server); err == nil {
		return addr.Zone() == ""
	}
	return len(server) <= 253 && hostPattern.MatchString(server)
}

func (cfg config) nodeByID(id string) (node, bool) {
	for _, entry := range cfg.Nodes {
		if entry.ID == id {
			return entry, true
		}
	}
	return node{}, false
}

func initializeConfig(path string) error {
	var cfg config
	if err := json.Unmarshal(exampleConfig, &cfg); err != nil {
		return err
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	cfg.Token = base64.RawURLEncoding.EncodeToString(token[:])
	for i := range cfg.Nodes {
		id, err := randomUUID()
		if err != nil {
			return err
		}
		cfg.Nodes[i].UUID = id
	}
	return writePrivateJSON(path, cfg)
}

func writePrivateJSON(path string, value any) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(contents, '\n'))
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

func randomUUID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}
