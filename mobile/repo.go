package mobile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	config "github.com/ipfs/kubo/config"
	serialize "github.com/ipfs/kubo/config/serialize"
)

func initConfig(path string, conf *config.Config) error {
	if configIsInitialized(path) {
		return nil
	}
	configFilename, err := config.Filename(path, "")
	if err != nil {
		return err
	}
	// initialization is the one time when it's okay to write to the config
	// without reading the config from disk and merging any user-provided keys
	// that may exist.
	if err := serialize.WriteConfigFile(configFilename, conf); err != nil {
		return err
	}

	return nil
}

func checkWritable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := filepath.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return errors.New(fmt.Sprintf("%s is not writeable by the current user", dir))
			}
			return errors.New(fmt.Sprintf("unexpected error while checking writeablility of repo root: %s", err))
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesn't exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return errors.New(fmt.Sprintf("cannot write to %s, incorrect permissions", err))
	}

	return err
}

func configIsInitialized(path string) bool {
	configFilename, err := config.Filename(path, "")
	if err != nil {
		return false
	}
	if fileExists(configFilename) {
		return true
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func openConfig(path string) (*config.Config, error) {
	configFilename, err := config.Filename(path, "")
	if err != nil {
		return nil, err
	}
	conf, err := serialize.Load(configFilename)
	if err != nil {
		return nil, err
	}

	return conf, err
}
