package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SteamSearchItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type SteamSearchResponse struct {
	Items []SteamSearchItem `json:"items"`
}

func searchSteamGame(name string) ([]SteamSearchItem, error) {
	url := fmt.Sprintf("https://store.steampowered.com/api/storesearch/?term=%s&l=english&cc=US", strings.ReplaceAll(name, " ", "+"))
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to search steam: %d", resp.StatusCode)
	}

	var searchResp SteamSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Items, nil
}

func downloadThumbnail(appID int, destPath string) error {
	// Try the portrait library image first, fallback to header
	urls := []string{
		fmt.Sprintf("https://steamcdn-a.akamaihd.net/steam/apps/%d/library_600x900.jpg", appID),
		fmt.Sprintf("https://steamcdn-a.akamaihd.net/steam/apps/%d/header.jpg", appID),
	}

	for _, url := range urls {
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			if err == nil {
				return nil // Successfully downloaded
			}
		}
	}
	return fmt.Errorf("failed to download thumbnail for app %d", appID)
}

func getProtonVersions() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{}
	}

	dir := filepath.Join(home, ".steam", "steam", "compatibilitytools.d")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	// Add some defaults in case
	if len(versions) == 0 {
		versions = append(versions, "Proton Experimental", "Proton 9.0-3")
	}
	return versions
}
