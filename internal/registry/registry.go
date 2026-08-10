package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
)

type file struct {
	Devices []config.Device `json:"devices"`
}

func Load(path string) ([]config.Device, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read device registry: %w", err)
	}
	var data file
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("decode device registry: %w", err)
	}
	for i := range data.Devices {
		data.Devices[i].MAC = config.NormalizeMAC(data.Devices[i].MAC)
	}
	return data.Devices, nil
}

func Save(path string, devices []config.Device) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(file{Devices: devices}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".devices-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
