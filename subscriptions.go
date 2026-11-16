package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

const routeGroup = "XiaYu Route"

func subscriptionHandler(cfg config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /sub/{token}/{id}/{format}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if subtle.ConstantTimeCompare([]byte(r.PathValue("token")), []byte(cfg.Token)) != 1 {
			http.NotFound(w, r)
			return
		}
		selected := cfg.Nodes
		if id := r.PathValue("id"); id != "all" {
			selected = nil
			for _, entry := range cfg.Nodes {
				if entry.ID == id {
					selected = []node{entry}
					break
				}
			}
		}
		if len(selected) == 0 {
			http.NotFound(w, r)
			return
		}
		switch r.PathValue("format") {
		case "mihomo.yaml":
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			writeMihomo(w, selected)
		case "sip008.json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			writeSIP008(w, selected)
		case "ss.txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writeShareLinks(w, selected)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

func writeMihomo(w io.Writer, nodes []node) {
	fmt.Fprint(w, "mixed-port: 7890\nallow-lan: false\nmode: rule\nproxies:\n")
	for _, entry := range nodes {
		fmt.Fprintf(w, "  - name: %q\n    type: ss\n    server: %q\n    port: %d\n    cipher: %q\n    password: %q\n    udp: %t\n",
			entry.Name, entry.Server, entry.Port, entry.Method, entry.Password, entry.UDP)
	}
	fmt.Fprintf(w, "proxy-groups:\n  - name: %q\n    type: select\n    proxies:\n", routeGroup)
	for _, entry := range nodes {
		fmt.Fprintf(w, "      - %q\n", entry.Name)
	}
	fmt.Fprintf(w, "rules:\n  - %q\n", "MATCH,"+routeGroup)
}

type sip008Server struct {
	UUID     string `json:"id"`
	Remarks  string `json:"remarks"`
	Server   string `json:"server"`
	Port     int    `json:"server_port"`
	Method   string `json:"method"`
	Password string `json:"password"`
}

func writeSIP008(w io.Writer, nodes []node) {
	document := struct {
		Version int            `json:"version"`
		Servers []sip008Server `json:"servers"`
	}{Version: 1, Servers: make([]sip008Server, 0, len(nodes))}
	for _, entry := range nodes {
		document.Servers = append(document.Servers, sip008Server{
			UUID:     entry.UUID,
			Remarks:  entry.Name,
			Server:   entry.Server,
			Port:     entry.Port,
			Method:   entry.Method,
			Password: entry.Password,
		})
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	encoder.Encode(document)
}

func writeShareLinks(w io.Writer, nodes []node) {
	for _, entry := range nodes {
		// SIP002 uses percent-encoded credentials with AEAD-2022.
		fmt.Fprintf(w, "ss://%s:%s@%s#%s\n",
			url.QueryEscape(entry.Method), url.QueryEscape(entry.Password),
			net.JoinHostPort(entry.Server, strconv.Itoa(entry.Port)), url.PathEscape(entry.Name))
	}
}
