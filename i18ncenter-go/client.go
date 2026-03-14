package i18ncenter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

// DeploymentStage represents the deployment stage
type DeploymentStage string

const (
	StageDraft      DeploymentStage = "draft"
	StageStaging    DeploymentStage = "staging"
	StageProduction DeploymentStage = "production"
)

// Config holds the client configuration
type Config struct {
	// APIBaseURL is the base URL of the i18n-center API (e.g., "https://api.example.com/api")
	APIBaseURL string
	// APIToken is the Bearer token for authentication (optional)
	APIToken string
	// DefaultLocale is the default locale to use (default: "en")
	DefaultLocale string
	// DefaultStage is the default deployment stage (default: "production")
	DefaultStage DeploymentStage
	// CacheTTL is the cache TTL duration (default: 1 hour)
	CacheTTL time.Duration
	// EnableCache enables caching (default: true)
	EnableCache bool
	// HTTPClient is a custom HTTP client (optional)
	HTTPClient *http.Client
}

// Client is the i18n-center API client
type Client struct {
	config     Config
	httpClient *http.Client
	cache      *cache.Cache
}

// TranslationData represents the translation JSON structure
type TranslationData map[string]interface{}

// NewClient creates a new i18n-center client
func NewClient(config Config) *Client {
	// Set defaults
	if config.DefaultLocale == "" {
		config.DefaultLocale = "en"
	}
	if config.DefaultStage == "" {
		config.DefaultStage = StageProduction
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = time.Hour
	}
	if config.EnableCache && config.CacheTTL > 0 {
		// EnableCache defaults to true
		if !config.EnableCache {
			config.EnableCache = true
		}
	}

	// Setup HTTP client
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Setup cache
	var c *cache.Cache
	if config.EnableCache {
		c = cache.New(config.CacheTTL, config.CacheTTL*2)
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		cache:      c,
	}
}

// GetTranslation fetches translation for a single component
// applicationCode is required to differentiate components with the same code in different applications
func (c *Client) GetTranslation(applicationCode string, componentCode string, locale string, stage DeploymentStage) (TranslationData, error) {
	if locale == "" {
		locale = c.config.DefaultLocale
	}
	if stage == "" {
		stage = c.config.DefaultStage
	}

	// Check cache
	if c.cache != nil {
		cacheKey := c.cacheKey(applicationCode, componentCode, locale, string(stage))
		if cached, found := c.cache.Get(cacheKey); found {
			return cached.(TranslationData), nil
		}
	}

	// Fetch from API
	translations, err := c.GetMultipleTranslations(applicationCode, []string{componentCode}, locale, stage)
	if err != nil {
		return nil, err
	}

	translation, ok := translations[componentCode]
	if !ok {
		return nil, fmt.Errorf("translation not found for component: %s", componentCode)
	}

	// Cache the result
	if c.cache != nil {
		cacheKey := c.cacheKey(applicationCode, componentCode, locale, string(stage))
		c.cache.Set(cacheKey, translation, c.config.CacheTTL)
	}

	return translation, nil
}

// GetMultipleTranslations fetches translations for multiple components at once
// applicationCode is required to differentiate components with the same code in different applications
func (c *Client) GetMultipleTranslations(applicationCode string, componentCodes []string, locale string, stage DeploymentStage) (map[string]TranslationData, error) {
	if locale == "" {
		locale = c.config.DefaultLocale
	}
	if stage == "" {
		stage = c.config.DefaultStage
	}

	// Check cache for all components
	results := make(map[string]TranslationData)
	missingCodes := []string{}

	if c.cache != nil {
		for _, code := range componentCodes {
			cacheKey := c.cacheKey(applicationCode, code, locale, string(stage))
			if cached, found := c.cache.Get(cacheKey); found {
				results[code] = cached.(TranslationData)
			} else {
				missingCodes = append(missingCodes, code)
			}
		}
	} else {
		missingCodes = componentCodes
	}

	// Fetch missing translations from API
	if len(missingCodes) > 0 {
		url := fmt.Sprintf("%s/translations/bulk?application_code=%s&component_codes=%s&locale=%s&stage=%s",
			c.config.APIBaseURL,
			applicationCode,
			c.joinCodes(missingCodes),
			locale,
			stage,
		)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add authorization header if token is provided
		if c.config.APIToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.config.APIToken)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch translations: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		var data map[string]TranslationData
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// Add to results and cache
		for _, code := range missingCodes {
			if translation, ok := data[code]; ok {
				results[code] = translation
				if c.cache != nil {
					cacheKey := c.cacheKey(applicationCode, code, locale, string(stage))
					c.cache.Set(cacheKey, translation, c.config.CacheTTL)
				}
			}
		}
	}

	return results, nil
}

// GetTranslationsByTag fetches translations for all components that have the given tag
// applicationID is the application UUID
func (c *Client) GetTranslationsByTag(applicationID string, tagCode string, locale string, stage DeploymentStage) (map[string]TranslationData, error) {
	if locale == "" {
		locale = c.config.DefaultLocale
	}
	if stage == "" {
		stage = c.config.DefaultStage
	}
	tagCode = strings.ToLower(strings.TrimSpace(tagCode))
	if tagCode == "" {
		return nil, fmt.Errorf("tag code is required")
	}

	cacheKey := fmt.Sprintf("bytag:%s:%s:%s:%s", applicationID, tagCode, locale, string(stage))
	if c.cache != nil {
		if cached, found := c.cache.Get(cacheKey); found {
			return cached.(map[string]TranslationData), nil
		}
	}

	url := fmt.Sprintf("%s/applications/%s/translations/by-tag/%s?locale=%s&stage=%s",
		c.config.APIBaseURL, applicationID, url.PathEscape(tagCode), locale, stage)
	data, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	result := make(map[string]TranslationData)
	if data != nil {
		for k, v := range data {
			if m, ok := v.(map[string]interface{}); ok {
				result[k] = m
			}
		}
	}
	if c.cache != nil {
		c.cache.Set(cacheKey, result, c.config.CacheTTL)
	}
	return result, nil
}

// GetTranslationsByPage fetches translations for all components that have the given page
// applicationID is the application UUID
func (c *Client) GetTranslationsByPage(applicationID string, pageCode string, locale string, stage DeploymentStage) (map[string]TranslationData, error) {
	if locale == "" {
		locale = c.config.DefaultLocale
	}
	if stage == "" {
		stage = c.config.DefaultStage
	}
	pageCode = strings.ToLower(strings.TrimSpace(pageCode))
	if pageCode == "" {
		return nil, fmt.Errorf("page code is required")
	}

	cacheKey := fmt.Sprintf("bypage:%s:%s:%s:%s", applicationID, pageCode, locale, string(stage))
	if c.cache != nil {
		if cached, found := c.cache.Get(cacheKey); found {
			return cached.(map[string]TranslationData), nil
		}
	}

	url := fmt.Sprintf("%s/applications/%s/translations/by-page/%s?locale=%s&stage=%s",
		c.config.APIBaseURL, applicationID, url.PathEscape(pageCode), locale, stage)
	data, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	result := make(map[string]TranslationData)
	if data != nil {
		for k, v := range data {
			if m, ok := v.(map[string]interface{}); ok {
				result[k] = m
			}
		}
	}
	if c.cache != nil {
		c.cache.Set(cacheKey, result, c.config.CacheTTL)
	}
	return result, nil
}

// ClearCache clears all cached translations
func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.Flush()
	}
}

// doGet performs a GET request and returns the response as map[string]interface{}
func (c *Client) doGet(url string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIToken)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return data, nil
}

// cacheKey generates a cache key (includes application code to differentiate)
func (c *Client) cacheKey(applicationCode, componentCode, locale, stage string) string {
	return fmt.Sprintf("i18n:%s:%s:%s:%s", applicationCode, componentCode, locale, stage)
}

// joinCodes joins component codes with comma
func (c *Client) joinCodes(codes []string) string {
	var buf bytes.Buffer
	for i, code := range codes {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(code)
	}
	return buf.String()
}

