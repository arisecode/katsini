package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
)

const DefaultTimeout = 30 * time.Second

var (
	browserMutex sync.Mutex
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
		// Use local Chrome
		browser = rod.New()
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

func HuaweiAppGallery(appID string) (App, error) {
	app := App{
		appID: appID,
		url:   fmt.Sprintf("https://appgallery.huawei.com/app/C%s", appID),
	}

	log.Printf("Fetching Huawei AppGallery app data for appID: %s", appID)

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
	if err := page.Navigate(app.url); err != nil {
		return App{}, fmt.Errorf("failed to navigate: %w", err)
	}
	page.MustWaitLoad()

	// wait for main content to load
	page.MustElement(`div[class="horizonhomecard"]`)
	page.MustElement(`div[class="componentContainer"]`)

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
		log.Printf("Error parsing date(%s): %s \n", updated, err)
		return App{}, err
	}

	app.updated = parsedDate.Format("02-01-2006")

	return app, nil
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
