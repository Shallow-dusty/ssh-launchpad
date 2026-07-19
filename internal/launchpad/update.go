package launchpad

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const latestReleaseAPI = "https://api.github.com/repos/Shallow-dusty/ssh-launchpad/releases/latest"

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	URL            string `json:"url"`
	Channel        string `json:"channel"`
}

// CheckForUpdate only reads release metadata. It never downloads or installs an update.
func CheckForUpdate(ctx context.Context) (UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ssh-launchpad/"+Version)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("check release metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("release metadata returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return UpdateInfo{}, err
	}
	latest := strings.TrimPrefix(payload.TagName, "v")
	return UpdateInfo{
		CurrentVersion: Version,
		LatestVersion:  latest,
		Available:      newerVersion(latest, Version),
		URL:            payload.HTMLURL,
		Channel:        "stable",
	}, nil
}

func newerVersion(candidate, current string) bool {
	parse := func(value string) [3]int {
		var result [3]int
		parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
		for index := 0; index < len(parts) && index < len(result); index++ {
			number := strings.SplitN(parts[index], "-", 2)[0]
			result[index], _ = strconv.Atoi(number)
		}
		return result
	}
	left, right := parse(candidate), parse(current)
	for index := range left {
		if left[index] != right[index] {
			return left[index] > right[index]
		}
	}
	return false
}
