package main

import (
	"encoding/json"
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const protectedConfigFormat = "dpapi-user-v1"

type protectedConfig struct {
	Format string `json:"format"`
	Data   []byte `json:"data"`
}

// DPAPI current-user scope; never CRYPTPROTECT_LOCAL_MACHINE. No password,
// shared application key or interactive prompt is used. This does not protect
// against malware already running as the same Windows user.
func protectConfig(data []byte, decrypt bool) ([]byte, error) {
	if len(data) == 0 || len(data) > 16<<20 {
		return nil, errors.New("invalid protected config size")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if decrypt {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, errors.New("Windows profile protection failed")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}

func decodeStoredConfig(data []byte) ([]byte, bool, error) {
	if len(data) > 16<<20 || !json.Valid(data) {
		return nil, false, errors.New("invalid stored config")
	}
	var envelope protectedConfig
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, false, err
	}
	if envelope.Format == "" {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil || object == nil {
			return nil, false, errors.New("config object required")
		}
		return data, true, nil
	}
	if envelope.Format != protectedConfigFormat {
		return nil, false, errors.New("unsupported config protection format")
	}
	plain, err := protectConfig(envelope.Data, true)
	if err != nil || !json.Valid(plain) {
		return nil, false, errors.New("cannot decrypt profile; original file retained")
	}
	return plain, false, nil
}

func saveProtectedConfig(path string, plain []byte) error {
	// Do not silently replace unreadable secrets with the empty defaults shown
	// after an error. Recovery/import requires explicitly moving that file aside.
	if existing, err := os.ReadFile(path); err == nil {
		decoded, _, err := decodeStoredConfig(existing)
		if err != nil {
			return err
		}
		var schema AppConfig
		if err := json.Unmarshal(decoded, &schema); err != nil {
			return errors.New("invalid existing config; original retained")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	ciphertext, err := protectConfig(plain, false)
	if err != nil {
		return err
	}
	data, err := json.Marshal(protectedConfig{Format: protectedConfigFormat, Data: ciphertext})
	if err != nil {
		return err
	}
	return saveGUIConfigFile(path, data)
}
