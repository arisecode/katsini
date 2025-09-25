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
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"

	"github.com/chromedp/chromedp"
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

func DisableFetchExceptScripts(ctx context.Context, resourceTypesToBlock []network.ResourceType) func(event any) {
	return func(event any) {
		if ev, ok := event.(*fetch.EventRequestPaused); ok {
			go func() {
				c := chromedp.FromContext(ctx)
				cdpCtx := cdp.WithExecutor(ctx, c.Target)

				shouldBlock := false
				for _, resourceType := range resourceTypesToBlock {
					if ev.ResourceType == resourceType {
						shouldBlock = true
						break
					}
				}

				if shouldBlock {
					if err := fetch.FailRequest(ev.RequestID, network.ErrorReasonBlockedByClient).Do(cdpCtx); err != nil {
						log.Printf("Failed to block request: %s \n", err)
						return
					}
				} else {
					if err := fetch.ContinueRequest(ev.RequestID).Do(cdpCtx); err != nil {
						log.Printf("Failed to continue request: %s \n", err)
						return
					}
				}
			}()
		}
	}
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

	// configure chromedp to use a remote Chrome instance
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), fmt.Sprintf("ws://%s:%s/json", os.Getenv("CHROME_HOST"), os.Getenv("CHROME_PORT")))
	defer cancel()

	// create chromedp context
	taskCtx, cancel := chromedp.NewContext(allocCtx) // enable debug log to see the CDP traffics.
	// chromedp.WithDebugf(log.Printf),
	defer cancel()

	chromedp.ListenTarget(taskCtx, DisableFetchExceptScripts(taskCtx, []network.ResourceType{
		network.ResourceTypeImage,
		network.ResourceTypeStylesheet,
		network.ResourceTypeFont,
		network.ResourceTypeMedia,
		network.ResourceTypeManifest,
		network.ResourceTypeOther,
	}))

	// set a timeout to avoid long waits
	timeoutCtx, cancel := context.WithTimeout(taskCtx, DefaultTimeout)
	defer cancel()

	xpath := `//div[contains(text(), "About this app") or contains(text(), "About this game")]`
	xpathTitle := `//div[contains(text(), "About this app") or contains(text(), "About this game")]/preceding-sibling::h5[1]`
	xpathVersion := ` //div[contains(text(), "Version")]/following-sibling::div[1]`
	xpathUpdated := `//div[contains(text(), "Updated")]/following-sibling::div[1]`
	xpathDeveloper := `//div[contains(text(), "Offered by")]/following-sibling::div[1]`

	var notFound bool
	var updated string

	// run the task to navigate and extract the version text
	if err := chromedp.Run(timeoutCtx,
		fetch.Enable(),
		chromedp.Navigate(app.url),
		// Check if app exists using JavaScript
		chromedp.Evaluate(`document.body.innerText.includes("We're sorry, the requested URL was not found on this server.")`, &notFound),
		chromedp.ActionFunc(func(_ context.Context) error {
			if notFound {
				return ErrAppNotFound
			}
			return nil
		}),
		// wait for the element is visible
		chromedp.WaitVisible(`button[aria-label="See more information on About this app"], button[aria-label="See more information on About this game"]`),
		// click the button
		chromedp.Click(`button[aria-label="See more information on About this app"], button[aria-label="See more information on About this game"]`),
		// wait for the element is visible
		chromedp.WaitVisible(xpath),
		// get app title
		chromedp.Text(xpathTitle, &app.title),
		// get app version
		chromedp.Text(xpathVersion, &app.version),
		// get app updated
		chromedp.Text(xpathUpdated, &updated),
		// get app developer
		chromedp.Text(xpathDeveloper, &app.developer),
	); err != nil {
		switch {
		case strings.Contains(err.Error(), "context deadline exceeded"):
			return App{}, fmt.Errorf("%w: timeout while extracting data", ErrPageLoad)
		case errors.Is(err, ErrAppNotFound):
			return App{}, ErrAppNotFound
		default:
			return App{}, fmt.Errorf("failed to extract app data: %w", err)
		}
	}

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
	// configure chromedp to use a remote Chrome instance
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), fmt.Sprintf("ws://%s:%s/json", os.Getenv("CHROME_HOST"), os.Getenv("CHROME_PORT")))
	defer cancel()

	// create chromedp context
	taskCtx, cancel := chromedp.NewContext(allocCtx) // enable debug log to see the CDP traffics.
	// chromedp.WithDebugf(log.Printf),
	defer cancel()

	chromedp.ListenTarget(taskCtx, DisableFetchExceptScripts(taskCtx, []network.ResourceType{
		network.ResourceTypeImage,
		network.ResourceTypeFont,
		network.ResourceTypeMedia,
		network.ResourceTypeManifest,
		network.ResourceTypeOther,
	}))

	// set a timeout to avoid long waits
	timeoutCtx, cancel := context.WithTimeout(taskCtx, DefaultTimeout)
	defer cancel()

	xpathVersion := ` //div[contains(text(), "Version")]/following-sibling::div[1]`
	xpathUpdated := `//div[contains(text(), "Updated")]/following-sibling::div[1]`
	xpathDeveloper := `//div[contains(text(), "Developer")]/following-sibling::div[1]`

	var notFound bool
	var updated string

	if err := chromedp.Run(timeoutCtx,
		fetch.Enable(),
		chromedp.Navigate(app.url),
		chromedp.WaitVisible(`div[class="horizonhomecard"]`),
		chromedp.WaitVisible(`div[class="componentContainer"]`),
		// check if app exists using JavaScript
		chromedp.Evaluate(`document.querySelector('.componentContainer').offsetHeight < 500`, &notFound),
		chromedp.ActionFunc(func(_ context.Context) error {
			if notFound {
				return ErrAppNotFound
			}
			return nil
		}),
		// wait for the element is visible to get the app title
		chromedp.Text(`div.center_info > div.title`, &app.title, chromedp.NodeVisible),
		// get app version
		chromedp.Text(xpathVersion, &app.version),
		// get app updated
		chromedp.Text(xpathUpdated, &updated),
		// get app developer
		chromedp.Text(xpathDeveloper, &app.developer),
		// get app package name
		chromedp.Evaluate(`document.querySelector('div[package]').getAttribute('package')`, &app.bundleID),
	); err != nil {
		switch {
		case strings.Contains(err.Error(), "context deadline exceeded"):
			return App{}, fmt.Errorf("%w: timeout while extracting data", ErrPageLoad)
		case errors.Is(err, ErrAppNotFound):
			return App{}, ErrAppNotFound
		default:
			return App{}, fmt.Errorf("failed to extract app data: %w", err)
		}
	}

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
