package controller

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/sya/internal/schema"
)

type VersionController struct {
	appVersion string
	githubRepo string
	cache      *versionCache
}

type versionCache struct {
	mu       sync.RWMutex
	version  string
	download string
	checked  time.Time
}

const versionCacheDuration = 24 * time.Hour

type VersionResponse struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	DownloadURL string `json:"download_url"`
	Changelog   string `json:"changelog"`
}

func NewVersionController(githubRepo string) *VersionController {
	version := "0.1.0"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				version = setting.Value[:8]
				break
			}
		}
	}
	return &VersionController{
		appVersion: version,
		githubRepo: githubRepo,
		cache:      &versionCache{},
	}
}

func (c *VersionController) RegisterRoutes(r *server.Hertz) {
	r.GET("/api/v1/version", c.CheckVersion)
	r.GET("/api/v1/desktop-version", c.DesktopVersion)
}

func (c *VersionController) CheckVersion(ctx context.Context, hc *app.RequestContext) {
	currentVersion := hc.Query("version")

	latestVersion := c.appVersion
	hasUpdate := c.hasNewerVersion(latestVersion, currentVersion)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"has_update": hasUpdate,
		"version":    latestVersion,
	}))
}

func (c *VersionController) DesktopVersion(ctx context.Context, hc *app.RequestContext) {
	currentVersion := hc.Query("version")

	latestVersion, downloadURL := c.getLatestVersionCached()

	hasUpdate := c.hasNewerVersion(latestVersion, currentVersion)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"has_update":      hasUpdate,
		"current_version": currentVersion,
		"latest_version":  latestVersion,
		"download_url":    downloadURL,
	}))
}

func (c *VersionController) getLatestVersionCached() (string, string) {
	c.cache.mu.RLock()
	if time.Since(c.cache.checked) < versionCacheDuration && c.cache.version != "" {
		c.cache.mu.RUnlock()
		return c.cache.version, c.cache.download
	}
	c.cache.mu.RUnlock()

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()

	if time.Since(c.cache.checked) >= versionCacheDuration {
		c.cache.version = c.appVersion
		c.cache.download = fmt.Sprintf("https://github.com/%s/releases", c.githubRepo)
		c.cache.checked = time.Now()
	}

	return c.cache.version, c.cache.download
}

func (c *VersionController) hasNewerVersion(latest, current string) bool {
	if current == "" || current == latest {
		return false
	}

	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		var latestNum, currentNum int
		fmt.Sscanf(latestParts[i], "%d", &latestNum)
		fmt.Sscanf(currentParts[i], "%d", &currentNum)

		if latestNum > currentNum {
			return true
		}
		if latestNum < currentNum {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}
