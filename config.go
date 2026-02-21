package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "umu-front")
	os.MkdirAll(configDir, 0755)
	return filepath.Join(configDir, "games.json")
}

func getImagesDir() string {
	home, _ := os.UserHomeDir()
	imgDir := filepath.Join(home, ".config", "umu-front", "images")
	os.MkdirAll(imgDir, 0755)
	return imgDir
}

func loadGames() ([]Game, error) {
	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Game{}, nil
		}
		return nil, err
	}
	var games []Game
	err = json.Unmarshal(data, &games)
	return games, err
}

func saveGames(games []Game) error {
	data, err := json.MarshalIndent(games, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0644)
}
