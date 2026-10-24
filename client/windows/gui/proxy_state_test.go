package main

import (
	"golang.org/x/sys/windows/registry"
	"testing"
)

type fakeProxyKey struct {
	enabled *uint32
	strings map[string]string
	kinds   map[string]uint32
}

func (k *fakeProxyKey) GetIntegerValue(string) (uint64, uint32, error) {
	if k.enabled == nil {
		return 0, 0, registry.ErrNotExist
	}
	return uint64(*k.enabled), registry.DWORD, nil
}
func (k *fakeProxyKey) GetStringValue(name string) (string, uint32, error) {
	if value, ok := k.strings[name]; ok {
		return value, k.kinds[name], nil
	}
	return "", 0, registry.ErrNotExist
}
func (k *fakeProxyKey) SetDWordValue(_ string, value uint32) error { k.enabled = &value; return nil }
func (k *fakeProxyKey) SetStringValue(name, value string) error {
	k.strings[name] = value
	k.kinds[name] = registry.SZ
	return nil
}
func (k *fakeProxyKey) SetExpandStringValue(name, value string) error {
	k.strings[name] = value
	k.kinds[name] = registry.EXPAND_SZ
	return nil
}
func (k *fakeProxyKey) DeleteValue(name string) error {
	if name == "ProxyEnable" {
		k.enabled = nil
	} else {
		delete(k.strings, name)
		delete(k.kinds, name)
	}
	return nil
}

func TestProxySnapshotRestoresPreviousProviderAndValueType(t *testing.T) {
	enabled := uint32(1)
	k := &fakeProxyKey{enabled: &enabled, strings: map[string]string{"ProxyServer": "%OTHER_PROXY%"}, kinds: map[string]uint32{"ProxyServer": registry.EXPAND_SZ}}
	snapshot, err := captureProxy(k)
	if err != nil {
		t.Fatal(err)
	}
	_ = k.SetStringValue("ProxyServer", "socks=127.0.0.1:1080")
	_ = k.SetStringValue("ProxyOverride", "localhost")
	if err := restoreProxy(k, snapshot); err != nil {
		t.Fatal(err)
	}
	if k.enabled == nil || *k.enabled != 1 || k.strings["ProxyServer"] != "%OTHER_PROXY%" || k.kinds["ProxyServer"] != registry.EXPAND_SZ {
		t.Fatal("previous proxy configuration was not restored")
	}
	if _, ok := k.strings["ProxyOverride"]; ok {
		t.Fatal("created a value absent from original settings")
	}
}

func TestProxySnapshotPreservesAbsentEnableValue(t *testing.T) {
	k := &fakeProxyKey{strings: map[string]string{}, kinds: map[string]uint32{}}
	snapshot, err := captureProxy(k)
	if err != nil {
		t.Fatal(err)
	}
	_ = k.SetDWordValue("ProxyEnable", 1)
	if err := restoreProxy(k, snapshot); err != nil {
		t.Fatal(err)
	}
	if k.enabled != nil {
		t.Fatal("did not restore missing ProxyEnable value")
	}
}
