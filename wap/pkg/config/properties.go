package config

import (
	"encoding/json"
	"os"
)

func ReadProperties() (map[string]interface{}, error) {
	content, err := os.ReadFile(CUSTOM_PROPERTIES_FILE)
	if err != nil {
		return nil, err
	}
	v := make(map[string]interface{})
	err = json.Unmarshal(content, &v)
	return v, err

}

func WriteProperties(p map[string]interface{}) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(CUSTOM_PROPERTIES_FILE, data, 0644)
}
