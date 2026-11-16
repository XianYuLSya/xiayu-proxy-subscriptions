package main

import (
	"errors"
	"fmt"
	"net/netip"
)

type singBoxConfig struct {
	Inbounds  []singBoxInbound  `json:"inbounds"`
	Outbounds []singBoxOutbound `json:"outbounds"`
	Route     map[string]string `json:"route"`
}

type singBoxInbound struct {
	Type     string `json:"type"`
	Tag      string `json:"tag"`
	Listen   string `json:"listen"`
	Port     int    `json:"listen_port"`
	Network  string `json:"network,omitempty"`
	Method   string `json:"method"`
	Password string `json:"password"`
}

type singBoxOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

func exportNodeConfig(cfg config, id, bindAddress, path string) error {
	entry, found := cfg.nodeByID(id)
	if !found {
		return fmt.Errorf("no node with ID %q", id)
	}
	addr, err := netip.ParseAddr(bindAddress)
	if err != nil || addr.Zone() != "" {
		return errors.New("-node-bind must be an IPv4 or IPv6 address without a port")
	}
	inbound := singBoxInbound{
		Type:     "shadowsocks",
		Tag:      "shadowsocks-in",
		Listen:   addr.String(),
		Port:     entry.Port,
		Method:   entry.Method,
		Password: entry.Password,
	}
	if !entry.UDP {
		inbound.Network = "tcp"
	}
	// A direct outbound opens its sockets on the host running sing-box.
	data := singBoxConfig{
		Inbounds:  []singBoxInbound{inbound},
		Outbounds: []singBoxOutbound{{Type: "direct", Tag: "direct"}},
		Route:     map[string]string{"final": "direct"},
	}
	return writePrivateJSON(path, data)
}
