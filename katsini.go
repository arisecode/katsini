package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type App struct {
	appID     string // Simple, unique identifier typically used within an app or system
	bundleID  string // Unique identifier for an entire app/application bundle
	url       string // URL of the app's page
	title     string // Title of the app
	version   string // Version of the app
	updated   string // Last updated date of the app
	developer string // Developer of the app
}

var (
	ErrAppNotFound = errors.New("app not found")
	ErrPageLoad    = errors.New("failed to load page")
	ErrDNSTimeout  = errors.New("DNS resolution timeout")
	ErrDNSFailed   = errors.New("DNS resolution failed")
)

const DefaultTimeout = 30 * time.Second

var (
	browserMutex  sync.Mutex
	sharedBrowser *rod.Browser
)

// getBrowser returns a shared browser instance or creates a new one
func getBrowser() *rod.Browser {
	browserMutex.Lock()
	defer browserMutex.Unlock()

	// If we already have a browser, try to reuse it
	if sharedBrowser != nil {
		return sharedBrowser
	}

	chromeHost := os.Getenv("CHROME_HOST")
	chromePort := os.Getenv("CHROME_PORT")

	var browser *rod.Browser
	if chromeHost != "" && chromePort != "" {
		// Use remote Chrome instance - create a RemoteURL to connect to existing Chrome
		u := launcher.MustResolveURL(fmt.Sprintf("http://%s:%s", chromeHost, chromePort))
		browser = rod.New().ControlURL(u)
	} else {
		// Use local Chrome with custom DNS configuration
		l := launcher.New()

		// Add VPS-optimized Chrome flags
		l = l.Set("--no-sandbox").
			Set("--disable-gpu").
			Set("--disable-dev-shm-usage").
			Set("--no-first-run").
			Set("--disable-background-timer-throttling").
			Set("--disable-backgrounding-occluded-windows").
			Set("--disable-renderer-backgrounding")

		// Configure custom DNS servers if provided
		customDNS := getCustomDNSServers()
		if len(customDNS) > 0 {
			dnsString := strings.Join(customDNS, ",")
			l = l.Set("--host-resolver-rules", fmt.Sprintf("MAP * 0.0.0.0, EXCLUDE %s", dnsString))
			log.Printf("Configured Chrome with custom DNS servers: %s", dnsString)
		}

		// Add specific DNS overrides for Huawei domains
		l = l.Set("--host-resolver-rules", "MAP appgallery.huawei.com 47.89.61.45")

		u := l.MustLaunch()
		browser = rod.New().ControlURL(u)
	}

	// Connect with error handling
	var err error
	for i := 0; i < 3; i++ {
		if err = browser.Connect(); err == nil {
			sharedBrowser = browser
			return sharedBrowser
		}
		log.Printf("Browser connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Second * 2)
	}

	// If all retries fail, panic as expected by the original MustConnect behavior
	panic(fmt.Sprintf("Failed to connect to browser after 3 attempts: %v", err))
}

func GooglePlayStore(bundleID, lang, country string) (App, error) {
	app := App{
		bundleID: bundleID,
	}

	if lang == "" {
		lang = "en"
	}

	if country == "" {
		country = "us"
	}

	log.Printf("Fetching Google Play Store app data for bundleID: %s, lang: %s, country: %s", bundleID, lang, country)
	app.url = fmt.Sprintf("https://play.google.com/store/apps/details?id=%s&hl=%s&gl=%s", app.bundleID, lang, country)

	// connect to remote Chrome instance
	// get shared browser instance
	browser := getBrowser()
	// Don't defer browser.MustClose() since it's shared

	// create a page with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	page := browser.Context(ctx).MustPage()
	defer page.MustClose()

	xpath := `//div[contains(text(), "About this app") or contains(text(), "About this game")]`
	xpathTitle := `//div[contains(text(), "About this app") or contains(text(), "About this game")]/preceding-sibling::h5[1]`
	xpathVersion := ` //div[contains(text(), "Version")]/following-sibling::div[1]`
	xpathUpdated := `//div[contains(text(), "Updated")]/following-sibling::div[1]`
	xpathDeveloper := `//div[contains(text(), "Offered by")]/following-sibling::div[1]`

	// navigate to the page
	if err := page.Navigate(app.url); err != nil {
		return App{}, fmt.Errorf("failed to navigate: %w", err)
	}
	page.MustWaitLoad()

	// check if app exists
	notFound, _ := page.Eval(`() => document.body.innerText.includes("We're sorry, the requested URL was not found on this server.")`)
	if notFound.Value.Bool() {
		return App{}, ErrAppNotFound
	}

	// wait for and click the "About this app" button
	buttonSelector := `button[aria-label="See more information on About this app"], button[aria-label="See more information on About this game"]`
	button := page.MustElement(buttonSelector)
	button.MustClick()

	// wait for the expanded section to appear
	page.MustElementX(xpath)

	// extract app information
	titleElement := page.MustElementX(xpathTitle)
	app.title = titleElement.MustText()

	versionElement := page.MustElementX(xpathVersion)
	app.version = versionElement.MustText()

	updatedElement := page.MustElementX(xpathUpdated)
	updated := updatedElement.MustText()

	developerElement := page.MustElementX(xpathDeveloper)
	app.developer = developerElement.MustText()

	parsedDate, err := time.Parse("Jan 2, 2006", updated)
	if err != nil {
		log.Printf("Error parsing date: %s \n", err)
		return App{}, err
	}
	app.updated = parsedDate.Format("02-01-2006")

	return app, nil
}

// checkDNSResolution tests DNS resolution for a given hostname
func checkDNSResolution(hostname string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Printf("Checking DNS resolution for %s", hostname)
	start := time.Now()

	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, hostname)

	duration := time.Since(start)
	log.Printf("DNS resolution for %s took %v", hostname, duration)

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("DNS resolution timeout for %s after %v", hostname, timeout)
		}
		return fmt.Errorf("DNS resolution failed for %s: %w", hostname, err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for %s", hostname)
	}

	log.Printf("DNS resolution successful for %s: found %d IP addresses", hostname, len(ips))
	return nil
}

// getCustomDNSServers returns custom DNS servers from environment variable
func getCustomDNSServers() []string {
	dnsServers := os.Getenv("DNS_SERVERS")
	if dnsServers == "" {
		return nil
	}
	return strings.Split(dnsServers, ",")
}

func HuaweiAppGallery(appID string) (App, error) {
	app := App{
		appID: appID,
		url:   fmt.Sprintf("https://appgallery.huawei.com/app/C%s", appID),
	}

	log.Printf("Fetching Huawei AppGallery app data for appID: %s", appID)

	// Check DNS resolution first to identify DNS vs other issues
	hostname := "appgallery.huawei.com"
	dnsTimeout := 10 * time.Second
	if err := checkDNSResolution(hostname, dnsTimeout); err != nil {
		log.Printf("DNS resolution failed for %s: %v", hostname, err)

		// Wrap the DNS error for better error reporting
		var dnsErr error
		if strings.Contains(err.Error(), "timeout") {
			dnsErr = fmt.Errorf("%w: %s", ErrDNSTimeout, err.Error())
		} else {
			dnsErr = fmt.Errorf("%w: %s", ErrDNSFailed, err.Error())
		}

		// Try fallback with token-based API if DNS fails
		log.Printf("Attempting fallback to token-based API for appID: %s", appID)
		fallbackApp, fallbackErr := HuaweiAppGalleryByToken(appID)
		if fallbackErr != nil {
			return App{}, fmt.Errorf("%w (fallback API error: %v)", dnsErr, fallbackErr)
		}
		log.Printf("Successfully retrieved app data via fallback API despite DNS issues")
		return fallbackApp, nil
	}

	// connect to remote Chrome instance
	// get shared browser instance
	browser := getBrowser()
	// Don't defer browser.MustClose() since it's shared

	// create a page with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	page := browser.Context(ctx).MustPage()
	defer page.MustClose()

	xpathVersion := ` //div[contains(text(), "Version")]/following-sibling::div[1]`
	xpathUpdated := `//div[contains(text(), "Updated")]/following-sibling::div[1]`
	xpathDeveloper := `//div[contains(text(), "Developer")]/following-sibling::div[1]`

	// navigate to the page
	log.Printf("Navigating to %s", app.url)
	start := time.Now()
	if err := page.Navigate(app.url); err != nil {
		duration := time.Since(start)
		log.Printf("Page navigation failed after %v: %v", duration, err)

		// Distinguish between timeout and other errors
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			return App{}, fmt.Errorf("%w: page navigation timeout after %v to %s", ErrPageLoad, duration, app.url)
		}
		return App{}, fmt.Errorf("%w: %s (took %v)", ErrPageLoad, err.Error(), duration)
	}

	navigationTime := time.Since(start)
	log.Printf("Page navigation completed in %v", navigationTime)

	page.MustWaitLoad()

	// wait for main content to load with timeout handling
	log.Printf("Waiting for main content elements to load")
	contentStart := time.Now()

	// Try to wait for main content elements, but capture a screenshot if they fail
	_, err := page.Timeout(15 * time.Second).Element(`div[class="horizonhomecards"]`)
	if err != nil {
		log.Printf("Failed to find horizonhomecard element: %v", err)
		if screenshot, shotErr := page.Screenshot(true, nil); shotErr != nil {
			log.Printf("Failed to capture page screenshot for debugging: %v", shotErr)
		} else {
			log.Printf("Page screenshot (base64): %s", base64.StdEncoding.EncodeToString(screenshot))
		}
		return App{}, fmt.Errorf("required page elements not found - page may not have loaded correctly")
	}

	_, err = page.Timeout(15 * time.Second).Element(`div[class="componentContainer"]`)
	if err != nil {
		log.Printf("Failed to find componentContainer element: %v", err)
		if screenshot, shotErr := page.Screenshot(true, nil); shotErr != nil {
			log.Printf("Failed to capture page screenshot for debugging: %v", shotErr)
		} else {
			log.Printf("Page screenshot (base64): %s", base64.StdEncoding.EncodeToString(screenshot))
		}
		return App{}, fmt.Errorf("required page elements not found - page may not have loaded correctly")
	}

	contentTime := time.Since(contentStart)
	log.Printf("Main content loaded in %v", contentTime)

	// check if app exists by checking container height
	containerHeight, _ := page.Eval(`() => document.querySelector('.componentContainer').offsetHeight`)
	if int(containerHeight.Value.Num()) < 500 {
		return App{}, ErrAppNotFound
	}

	// extract app information
	titleElement := page.MustElement(`div.center_info > div.title`)
	app.title = titleElement.MustText()

	versionElement := page.MustElementX(xpathVersion)
	app.version = versionElement.MustText()

	updatedElement := page.MustElementX(xpathUpdated)
	updated := updatedElement.MustText()

	developerElement := page.MustElementX(xpathDeveloper)
	app.developer = developerElement.MustText()

	// get app package name
	packageName, _ := page.Eval(`() => document.querySelector('div[package]').getAttribute('package')`)
	app.bundleID = packageName.Value.Str()

	parsedDate, err := time.Parse("1/2/2006", updated)
	if err != nil {
		log.Printf("Error parsing date(%s): %s", updated, err)
		return App{}, err
	}

	app.updated = parsedDate.Format("02-01-2006")

	return app, nil
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func AppleAppStore(appID, bundleID, country string) (App, error) {
	if country == "" {
		country = "us"
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	itunesURL := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&country=%s", appID, country)
	if bundleID != "" {
		itunesURL = fmt.Sprintf("https://itunes.apple.com/lookup?bundleId=%s&country=%s", bundleID, country)
	}

	log.Printf("Fetching AppleAppStore app data for appID: %s", appID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, itunesURL, http.NoBody)
	if err != nil {
		return App{}, err
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return App{}, fmt.Errorf("failed to get app: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return App{}, ErrAppNotFound
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		return App{}, err
	}

	var response struct {
		ResultCount int `json:"resultCount"`
		Results     []struct {
			Version                   string `json:"version"`
			CurrentVersionReleaseDate string `json:"currentVersionReleaseDate"`
			BundleID                  string `json:"bundleId"`
			TrackID                   int    `json:"trackId"`
			TrackName                 string `json:"trackName"`
			TrackViewURL              string `json:"trackViewUrl"`
			ArtistName                string `json:"artistName"`
		}
	}

	if err = json.Unmarshal(body, &response); err != nil {
		log.Printf("Failed to unmarshal body: %v", err)
		return App{}, err
	}

	if response.ResultCount == 0 {
		return App{}, fmt.Errorf("no app found")
	}

	parseDate, err := time.Parse("2006-01-02T15:04:05Z", response.Results[0].CurrentVersionReleaseDate)
	if err != nil {
		log.Printf("Error parsing date: %s \n", err)
		return App{}, err
	}

	return App{
		appID:     strconv.Itoa(response.Results[0].TrackID),
		bundleID:  response.Results[0].BundleID,
		url:       response.Results[0].TrackViewURL,
		title:     response.Results[0].TrackName,
		version:   response.Results[0].Version,
		updated:   parseDate.Format("02-01-2006"),
		developer: response.Results[0].ArtistName,
	}, nil
}

func HuaweiAppGalleryByToken(appID string) (App, error) {
	token, err := getHuaweiToken()
	if err != nil {
		return App{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://connect-api.cloud.huawei.com/api/publish/v2/app-info?appId=%s", appID), http.NoBody)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return App{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("client_id", os.Getenv("HUAWEI_CLIENT_ID"))

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to get app from Huawei AppGallery: %v", err)
		return App{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Status code is not OK: %d", resp.StatusCode)
		return App{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		return App{}, err
	}

	var appResponse struct {
		AppInfo struct {
			VersionNumber string `json:"versionNumber"`
			UpdateTime    string `json:"updateTime"`
		}
		Languages []struct {
			AppName string `json:"appName"`
		} `json:"languages"`
	}

	if err := json.Unmarshal(body, &appResponse); err != nil {
		log.Printf("Failed to unmarshal body: %v", err)
		return App{}, err
	}

	parseDate, err := time.Parse("2006-01-02 15:04:05", appResponse.AppInfo.UpdateTime)
	if err != nil {
		log.Printf("Error parsing date: %s \n", err)
		return App{}, err
	}

	return App{
		bundleID: appID,
		title:    appResponse.Languages[0].AppName,
		version:  appResponse.AppInfo.VersionNumber,
		updated:  parseDate.Format("02-01-2006"),
	}, nil
}

func getHuaweiToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     os.Getenv("HUAWEI_CLIENT_ID"),
		"client_secret": os.Getenv("HUAWEI_CLIENT_SECRET"),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Marshal failed: %v", err)
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://connect-api-dre.cloud.huawei.com/api/oauth2/v1/token", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Faield to create request: %v", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to get response: %v", err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Status code is not OK: %d", resp.StatusCode)
		return "", fmt.Errorf("status code is not OK: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		return "", err
	}

	// Parse the response using a struct
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		log.Printf("Failed to unmarshal body: %v", err)
		return "", err
	}

	return tokenResponse.AccessToken, nil
}
