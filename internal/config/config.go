package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	DbURL           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func Read() (Config, error) {
	configPath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}

	fileContent, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	var data Config
	if err := json.Unmarshal(fileContent, &data); err != nil {
		return Config{}, err
	}
	return data, nil
}

func Write(config Config) error {
	configPath, err := getConfigFilePath()
	if err != nil {
		return err
	}

	jsonConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, jsonConfig, 0o666); err != nil {
		return err
	}
	return nil
}

func getConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/" + ".gatorconfig.json", nil
}
