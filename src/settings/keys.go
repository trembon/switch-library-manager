package settings

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/magiconair/properties"
	"go.uber.org/zap"
)

var (
	keysInstance *switchKeys
)

type switchKeys struct {
	keys map[string]string
}

func (k *switchKeys) GetKey(keyName string) string {
	return k.keys[keyName]
}

func SwitchKeys() (*switchKeys, error) {
	return keysInstance, nil
}

func InitSwitchKeys(baseFolder string) (*switchKeys, error) {
	var (
		path string
		p *properties.Properties
		err error
	)
	logger := zap.S()

	// first, try to read the prod keys from the settings value
	settings := ReadSettings(baseFolder)
	if settings.Prodkeys != "" {
		path = settings.Prodkeys
		if !strings.HasSuffix(path, ".keys") {
			path = filepath.Join(path, "prod.keys")
		}

		logger.Infof("Trying to load prod.keys based on settings.json: %v", path)
		p, err = properties.LoadFile(path, properties.UTF8)
	}else{
		err = errors.New("prod.keys not defined in settings.json")
	}

	// second, if not found by settings look into the current folder
	if err != nil {
		path = filepath.Join(baseFolder, "prod.keys")

		logger.Infof("Trying to load prod.keys based on current folder: %v", path)
		p, err = properties.LoadFile(path, properties.UTF8)
	}

	// third, if not found in current, look in home directory
	if err != nil {
		path = "${HOME}/.switch/prod.keys"

		logger.Infof("Trying to load prod.keys based on home directory: %v", path)
		p, err = properties.LoadFile(path, properties.UTF8)
	}

	if err != nil {
		logger.Info("Unable to find prod.keys")
		return nil, errors.New("Error trying to read prod.keys [reason:" + err.Error() + "]")
	}

	keysInstance = &switchKeys{keys: map[string]string{}}
	for _, key := range p.Keys() {
		value, _ := p.Get(key)
		keysInstance.keys[key] = value
	}

	logger.Infof("Loaded prod.keys from: %v", path)
	return keysInstance, nil
}
