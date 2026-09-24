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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"

	undetected "github.com/Davincible/chromedp-undetected"
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

// Common resource types to block for faster page loading
var commonResourceTypesToBlock = []network.ResourceType{
	network.ResourceTypeImage,
	network.ResourceTypeFont,
	network.ResourceTypeMedia,
	network.ResourceTypeManifest,
	network.ResourceTypeOther,
}

// createBrowserContext creates a browser context with anti-bot protection
// It automatically detects whether to use local Chrome or remote Chrome based on environment variables
func createBrowserContext() (context.Context, context.CancelFunc, error) {
	chromeHost := os.Getenv("CHROME_HOST")
	chromePort := os.Getenv("CHROME_PORT")

	// If CHROME_HOST and CHROME_PORT are set, use remote Chrome (for backward compatibility with tests)
	if chromeHost != "" && chromePort != "" {
		log.Printf("Using remote Chrome at %q:%q", chromeHost, chromePort) // #nosec G706 -- user input is escaped with %q
		// Try undetected mode with remote Chrome
		taskCtx, cancel, err := undetected.New(undetected.Config{
			ChromePath: "ws://" + chromeHost + ":" + chromePort,
			Headless:   true,
			NoSandbox:  true,
		})
		if err != nil {
			log.Printf("Undetected mode not available, falling back to regular chromedp: %v", err)
			// Fallback to regular chromedp
			allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), fmt.Sprintf("ws://%s:%s/json", chromeHost, chromePort))
			taskCtx, cancel = chromedp.NewContext(allocCtx)
			// Return a combined cancel function
			return taskCtx, func() {
				cancel()
				allocCancel()
			}, nil
		}
		return taskCtx, cancel, nil
	}

	// Use local Chrome with chromedp-undetected
	log.Printf("Using local Chrome with chromedp-undetected")
	taskCtx, cancel, err := undetected.New(undetected.Config{
		Headless:  true,
		NoSandbox: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create undetected context: %w", err)
	}
	return taskCtx, cancel, nil
}

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

	log.Printf("Fetching Google Play Store app data for bundleID: %q, lang: %q, country: %q", bundleID, lang, country) // #nosec G706 -- user input is escaped with %q
	app.url = fmt.Sprintf("https://play.google.com/store/apps/details?id=%s&hl=%s&gl=%s",
		url.QueryEscape(bundleID), url.QueryEscape(lang), url.QueryEscape(country))

	// Create context with chromedp-undetected for anti-bot protection
	// Automatically uses local Chrome or falls back to remote if configured
	taskCtx, cancel, err := createBrowserContext()
	if err != nil {
		return App{}, fmt.Errorf("failed to create browser context: %w", err)
	}
	defer cancel()

	chromedp.ListenTarget(taskCtx, DisableFetchExceptScripts(taskCtx, append(commonResourceTypesToBlock, network.ResourceTypeStylesheet)))

	// set a timeout to avoid long waits
	timeoutCtx, cancel := context.WithTimeout(taskCtx, DefaultTimeout)
	defer cancel()

	aboutButton := `button[aria-label="See more information on About this app"], button[aria-label="See more information on About this game"]`
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
		chromedp.WaitVisible(aboutButton),
		// click the button via JS: a mouse click at its coordinates can land on the
		// overlapping header (e.g. the "Games" tab) and navigate away
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector(%q).click()`, aboutButton), nil),
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

// parseFlexibleDate attempts to parse a date string using multiple common formats
func parseFlexibleDate(dateStr string) (time.Time, error) {
	formats := []string{
		"1/2/2006",            // Huawei format (M/D/YYYY)
		"2/1/2006",            // Alternative format (D/M/YYYY)
		"2006-01-02",          // ISO format
		"Jan 2, 2006",         // Google Play format
		"2006-01-02 15:04:05", // Huawei API format
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date '%s' with known formats", dateStr)
}

// validateAppData checks that critical app fields are populated
func validateAppData(app *App, source string) error {
	if app.title == "" {
		return fmt.Errorf("%s: missing app title", source)
	}
	if app.version == "" {
		return fmt.Errorf("%s: missing app version", source)
	}
	if app.bundleID == "" {
		return fmt.Errorf("%s: missing bundle ID", source)
	}
	return nil
}

// retryOperation retries a function with exponential backoff
func retryOperation(operation func() (App, error), maxRetries int, operationName string) (App, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * time.Second
			log.Printf("%q: retry attempt %d/%d after %v", operationName, attempt+1, maxRetries, backoff) // #nosec G706 -- user input is escaped with %q
			time.Sleep(backoff)
		}

		app, err := operation()
		if err == nil {
			if attempt > 0 {
				log.Printf("%q: succeeded on attempt %d/%d", operationName, attempt+1, maxRetries) // #nosec G706 -- user input is escaped with %q
			}
			return app, nil
		}

		if errors.Is(err, ErrAppNotFound) {
			return App{}, err
		}

		lastErr = err
	}
	return App{}, fmt.Errorf("%s: failed after %d attempts: %w", operationName, maxRetries, lastErr)
}

func HuaweiAppGallery(appID string) (App, error) {
	// The web API is plain HTTP (no browser), so it is fast and works from hosts
	// where headless Chrome gets an empty page
	app, err := huaweiAppGalleryWebAPI("app|C" + appID)
	if err == nil {
		return app, nil
	}
	if errors.Is(err, ErrAppNotFound) {
		return App{}, err
	}
	log.Printf("Huawei AppGallery web API failed for appID %q, falling back to scraping: %v", appID, err) // #nosec G706 -- user input is escaped with %q

	app, err = retryOperation(func() (App, error) {
		return huaweiAppGalleryScrape(appID)
	}, 3, fmt.Sprintf("HuaweiAppGallery scrape for appID %s", appID))

	if err == nil {
		return app, nil
	}

	if shouldUseHuaweiAPIFallback() {
		log.Printf("Falling back to Huawei AppGallery API for appID %q due to scrape error: %v", appID, err) // #nosec G706 -- user input is escaped with %q
		if fallback, apiErr := HuaweiAppGalleryByToken(appID); apiErr == nil {
			return fallback, nil
		} else {
			log.Printf("Huawei AppGallery API fallback failed: %v", apiErr)
		}
	}

	return App{}, err
}

func shouldUseHuaweiAPIFallback() bool {
	return os.Getenv("HUAWEI_CLIENT_ID") != "" && os.Getenv("HUAWEI_CLIENT_SECRET") != ""
}

const huaweiWebAPIBase = "https://web-dra.hispace.dbankcloud.com/edge"

// HuaweiAppGalleryByBundleID looks up an app by its package name (e.g. com.example.app)
func HuaweiAppGalleryByBundleID(bundleID string) (App, error) {
	return huaweiAppGalleryWebAPI("package|" + bundleID)
}

// huaweiAppGalleryWebAPI fetches app data from the JSON API behind the AppGallery website.
// uri selects the app: "app|C<appID>" or "package|<bundleID>".
func huaweiAppGalleryWebAPI(uri string) (App, error) {
	log.Printf("Fetching Huawei AppGallery app data via web API for uri: %q", uri) // #nosec G706 -- user input is escaped with %q

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	// The detail endpoint requires a short-lived interface code issued by the site
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, huaweiWebAPIBase+"/webedge/getInterfaceCode",
		strings.NewReader(`{"params":{},"zone":"","locale":"en_US"}`))
	if err != nil {
		return App{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var interfaceCode string
	if err := doHuaweiWebRequest(req, &interfaceCode); err != nil {
		return App{}, fmt.Errorf("failed to get interface code: %w", err)
	}
	if interfaceCode == "" {
		return App{}, errors.New("empty interface code")
	}

	params := url.Values{
		"method":      {"internal.getTabDetail"},
		"serviceType": {"20"},
		"reqPageNum":  {"1"},
		"maxResults":  {"25"},
		"uri":         {uri},
		"zone":        {""},
		"locale":      {"en_US"},
	}
	detailURL := huaweiWebAPIBase + "/uowap/index?" + params.Encode()
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, detailURL, http.NoBody)
	if err != nil {
		return App{}, err
	}
	req.Header.Set("Interface-Code", fmt.Sprintf("%s_%d", interfaceCode, time.Now().UnixMilli()))

	var detail struct {
		RtnCode    int    `json:"rtnCode"`
		RtnDesc    string `json:"rtnDesc"`
		LayoutData []struct {
			DataList []struct {
				Name        string `json:"name"`
				Package     string `json:"package"`
				AppID       string `json:"appid"`
				VersionName string `json:"versionName"`
				Developer   string `json:"developer"`
				ReleaseDate string `json:"releaseDate"`
			} `json:"dataList"`
		} `json:"layoutData"`
	}
	if err := doHuaweiWebRequest(req, &detail); err != nil {
		return App{}, fmt.Errorf("failed to get app detail: %w", err)
	}
	if detail.RtnCode != 0 {
		return App{}, fmt.Errorf("huawei web api returned code %d: %s", detail.RtnCode, detail.RtnDesc)
	}

	var app App
	var updated string

	// App fields are spread across several detail cards; the card carrying
	// versionName holds the title, and the app info card holds developer and date
	for _, layout := range detail.LayoutData {
		for _, item := range layout.DataList {
			if item.VersionName != "" && app.version == "" {
				app.title = item.Name
				app.version = item.VersionName
				app.bundleID = item.Package
				app.appID = strings.TrimPrefix(item.AppID, "C")
			}
			if item.Developer != "" && app.developer == "" {
				app.developer = item.Developer
				updated = item.ReleaseDate
			}
		}
	}

	// Delisted or unknown apps return an empty detail page
	if app.version == "" && app.developer == "" {
		return App{}, ErrAppNotFound
	}

	if err := validateAppData(&app, "Huawei AppGallery web API"); err != nil {
		return App{}, err
	}
	if app.appID == "" {
		return App{}, errors.New("huawei AppGallery web API: missing app ID")
	}
	app.url = "https://appgallery.huawei.com/app/C" + url.PathEscape(app.appID)

	parsedDate, err := parseFlexibleDate(updated)
	if err != nil {
		return App{}, fmt.Errorf("failed to parse update date: %w", err)
	}
	app.updated = parsedDate.Format("02-01-2006")

	return app, nil
}

func doHuaweiWebRequest(req *http.Request, out any) error {
	req.Header.Set("Origin", "https://appgallery.huawei.com")
	req.Header.Set("Referer", "https://appgallery.huawei.com/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code is not OK: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func huaweiAppGalleryScrape(appID string) (App, error) {
	app := App{
		appID: appID,
		url:   "https://appgallery.huawei.com/app/C" + url.PathEscape(appID),
	}

	log.Printf("Fetching Huawei AppGallery app data for appID: %q", appID) // #nosec G706 -- user input is escaped with %q

	// Create context with chromedp-undetected for anti-bot protection
	taskCtx, cancel, err := createBrowserContext()
	if err != nil {
		return App{}, fmt.Errorf("failed to create browser context: %w", err)
	}
	defer cancel()

	// Use shared resource blocking configuration
	chromedp.ListenTarget(taskCtx, DisableFetchExceptScripts(taskCtx, commonResourceTypesToBlock))

	timeoutCtx, cancel := context.WithTimeout(taskCtx, DefaultTimeout)
	defer cancel()

	var notFound bool
	var updated string

	// Structure to hold all extracted data from JavaScript
	var extractedData struct {
		Title     string `json:"title"`
		Version   string `json:"version"`
		Updated   string `json:"updated"`
		Developer string `json:"developer"`
		BundleID  string `json:"bundleID"`
	}

	if err := chromedp.Run(timeoutCtx,
		fetch.Enable(),
		chromedp.Navigate(app.url),
		chromedp.WaitVisible(`div[class="horizonhomecard"]`),
		chromedp.WaitVisible(`div[class="componentContainer"]`),
		// Check if app exists by examining component container height
		// A height < 500px typically indicates an error or missing app page
		chromedp.Evaluate(`document.querySelector('.componentContainer').offsetHeight < 500`, &notFound),
		chromedp.ActionFunc(func(_ context.Context) error {
			if notFound {
				return ErrAppNotFound
			}
			return nil
		}),

		chromedp.Evaluate(`
			(function() {
				const getTextByXPath = (xpath) => {
					const result = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
					return result.singleNodeValue?.innerText?.trim() || '';
				};

				return {
					title: document.querySelector('div.center_info > div.title')?.innerText?.trim() || '',
					version: getTextByXPath('//div[contains(text(), "Version")]/following-sibling::div[1]'),
					updated: getTextByXPath('//div[contains(text(), "Updated")]/following-sibling::div[1]'),
					developer: getTextByXPath('//div[contains(text(), "Developer")]/following-sibling::div[1]'),
					bundleID: document.querySelector('div[package]')?.getAttribute('package') || ''
				};
			})()
		`, &extractedData),
	); err != nil {
		switch {
		case strings.Contains(err.Error(), "context deadline exceeded"):
			return App{}, fmt.Errorf("%w: timeout while extracting data from %s", ErrPageLoad, app.url)
		case errors.Is(err, ErrAppNotFound):
			return App{}, ErrAppNotFound
		default:
			return App{}, fmt.Errorf("failed to extract app data from %s: %w", app.url, err)
		}
	}

	app.title = extractedData.Title
	app.version = extractedData.Version
	app.developer = extractedData.Developer
	app.bundleID = extractedData.BundleID
	updated = extractedData.Updated

	if err := validateAppData(&app, "Huawei AppGallery scrape"); err != nil {
		return App{}, err
	}

	parsedDate, err := parseFlexibleDate(updated)
	if err != nil {
		log.Printf("Error parsing date %q: %v", updated, err)
		return App{}, fmt.Errorf("failed to parse update date: %w", err)
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

	params := url.Values{"id": {appID}, "country": {country}}
	if bundleID != "" {
		params = url.Values{"bundleId": {bundleID}, "country": {country}}
	}
	itunesURL := "https://itunes.apple.com/lookup?" + params.Encode()

	log.Printf("Fetching AppleAppStore app data for appID: %q, bundleID: %q", appID, bundleID) // #nosec G706 -- user input is escaped with %q
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
		Results []struct {
			Version                   string `json:"version"`
			CurrentVersionReleaseDate string `json:"currentVersionReleaseDate"`
			BundleID                  string `json:"bundleId"`
			TrackName                 string `json:"trackName"`
			TrackViewURL              string `json:"trackViewUrl"`
			ArtistName                string `json:"artistName"`
			TrackID                   int    `json:"trackId"`
		}
		ResultCount int `json:"resultCount"`
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
		"https://connect-api.cloud.huawei.com/api/publish/v2/app-info?"+url.Values{"appId": {appID}}.Encode(), http.NoBody)
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
		return App{}, fmt.Errorf("status code is not OK: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		return App{}, err
	}

	var appResponse struct {
		Ret struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		} `json:"ret"`
		AppInfo struct {
			AppName       string `json:"appName"`
			PackageName   string `json:"packageName"`
			VersionNumber string `json:"versionNumber"`
			UpdateTime    string `json:"updateTime"`
			DeveloperName string `json:"developerName"`
		} `json:"appInfo"`
		Languages []struct {
			AppName  string `json:"appName"`
			Language string `json:"language"`
		} `json:"languages"`
	}

	if err := json.Unmarshal(body, &appResponse); err != nil {
		log.Printf("Failed to unmarshal body: %v", err)
		return App{}, err
	}

	if appResponse.Ret.Code != "" && appResponse.Ret.Code != "0" {
		return App{}, fmt.Errorf("huawei api returned error code %s: %s", appResponse.Ret.Code, appResponse.Ret.Msg)
	}

	parseDate, err := time.Parse("2006-01-02 15:04:05", appResponse.AppInfo.UpdateTime)
	if err != nil {
		log.Printf("Error parsing date: %s \n", err)
		return App{}, err
	}

	title := appResponse.AppInfo.AppName
	if title == "" && len(appResponse.Languages) > 0 {
		title = appResponse.Languages[0].AppName
	}

	bundleID := appResponse.AppInfo.PackageName
	if bundleID == "" {
		bundleID = appID
	}

	return App{
		appID:     appID,
		bundleID:  bundleID,
		url:       "https://appgallery.huawei.com/app/C" + url.PathEscape(appID),
		title:     title,
		version:   appResponse.AppInfo.VersionNumber,
		updated:   parseDate.Format("02-01-2006"),
		developer: appResponse.AppInfo.DeveloperName,
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
		log.Printf("Failed to create request: %v", err)
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
