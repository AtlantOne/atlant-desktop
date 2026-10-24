package main

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows/registry"
)

type proxyKey interface {
	GetIntegerValue(string) (uint64, uint32, error)
	GetStringValue(string) (string, uint32, error)
	SetDWordValue(string, uint32) error
	SetStringValue(string, string) error
	SetExpandStringValue(string, string) error
	DeleteValue(string) error
}

type savedProxyValue struct {
	present bool
	text    string
	kind    uint32
}
type proxySnapshot struct {
	enabled          uint32
	hasEnabled       bool
	server, override savedProxyValue
}

var originalProxy *proxySnapshot // guarded by cmdMu, like the core process

func captureProxy(k proxyKey) (*proxySnapshot, error) {
	snapshot := &proxySnapshot{}
	value, kind, err := k.GetIntegerValue("ProxyEnable")
	if err == nil {
		if kind != registry.DWORD {
			return nil, fmt.Errorf("unsupported ProxyEnable type")
		}
		snapshot.enabled, snapshot.hasEnabled = uint32(value), true
	} else if !errors.Is(err, registry.ErrNotExist) {
		return nil, err
	}
	for name, target := range map[string]*savedProxyValue{"ProxyServer": &snapshot.server, "ProxyOverride": &snapshot.override} {
		text, kind, err := k.GetStringValue(name)
		if err == nil {
			*target = savedProxyValue{true, text, kind}
		} else if !errors.Is(err, registry.ErrNotExist) {
			return nil, err
		}
	}
	return snapshot, nil
}

func restoreProxy(k proxyKey, snapshot *proxySnapshot) error {
	for name, value := range map[string]savedProxyValue{"ProxyServer": snapshot.server, "ProxyOverride": snapshot.override} {
		var err error
		if !value.present {
			err = k.DeleteValue(name)
		} else if value.kind == registry.EXPAND_SZ {
			err = k.SetExpandStringValue(name, value.text)
		} else {
			err = k.SetStringValue(name, value.text)
		}
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	if snapshot.hasEnabled {
		return k.SetDWordValue("ProxyEnable", snapshot.enabled)
	}
	err := k.DeleteValue("ProxyEnable")
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}
