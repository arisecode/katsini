package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Huawei test app shared by the katsini and handler tests
const (
	checker5gBundleID  = "com.scriptrepublic.checker5g"
	checker5gTitle     = "5G Checker"
	checker5gURL       = "https://appgallery.huawei.com/app/C103228579"
	checker5gDeveloper = "ScriptRepublic"
)

func TestGooglePlayStore(t *testing.T) {
	testCases := []struct {
		bundleID  string
		title     string
		url       string
		developer string
	}{
		{
			bundleID:  "com.gianlu.timeless",
			title:     "Timeless",
			url:       "https://play.google.com/store/apps/details?id=com.gianlu.timeless&hl=en&gl=us",
			developer: "devgianlu",
		},
		{
			bundleID:  "com.burakgon.dnschanger",
			title:     "DNS Changer",
			url:       "https://play.google.com/store/apps/details?id=com.burakgon.dnschanger&hl=en&gl=us",
			developer: "BGN CAPITAL",
		},
		{
			bundleID:  "pro.flutters.app",
			title:     "Codalingo: Learn to Code",
			url:       "https://play.google.com/store/apps/details?id=pro.flutters.app&hl=en&gl=us",
			developer: "develooper.io",
		},
		{
			bundleID:  "com.simplemobiletools.notes.pro",
			title:     "Simple Notes Pro",
			url:       "https://play.google.com/store/apps/details?id=com.simplemobiletools.notes.pro&hl=en&gl=us",
			developer: "Simple Mobile Tool",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			app, err := GooglePlayStore(tc.bundleID, "en", "us")
			assert.NoError(t, err)
			assert.Equal(t, tc.title, app.title)
			assert.Equal(t, tc.url, app.url)
			assert.Equal(t, tc.developer, app.developer)
		})
	}
}

func TestParsePlayStorePage(t *testing.T) {
	// details sits at data[1][2]; build it with the indexes Play uses
	details := make([]any, 146)
	details[0] = []any{"Beyondium"}
	details[68] = []any{"Mediocre"}
	details[140] = []any{[]any{[]any{"1.1.5"}}}
	details[145] = []any{[]any{"Oct 31, 2019", []any{1572533883, 121000000}}}
	data, err := json.Marshal([]any{nil, []any{nil, nil, details}})
	assert.NoError(t, err)

	page := "<script>AF_initDataCallback({key: 'ds:0', hash: '1', data:[1,2], sideChannel: {}});</script>" +
		"<script>AF_initDataCallback({key: 'ds:5', hash: '2', data:" + string(data) + ", sideChannel: {}});</script>"

	var app App
	assert.NoError(t, parsePlayStorePage([]byte(page), &app))
	assert.Equal(t, "Beyondium", app.title)
	assert.Equal(t, "Mediocre", app.developer)
	assert.Equal(t, "1.1.5", app.version)
	assert.Equal(t, "31-10-2019", app.updated)

	assert.Error(t, parsePlayStorePage([]byte("<html></html>"), &App{}))
	assert.Error(t, parsePlayStorePage([]byte("<script>AF_initDataCallback({key: 'ds:1', hash: '1'});</script>"), &App{}))

	details[145] = nil
	data, err = json.Marshal([]any{nil, []any{nil, nil, details}})
	assert.NoError(t, err)
	page = "<script>AF_initDataCallback({key: 'ds:5', hash: '2', data:" + string(data) + ", sideChannel: {}});</script>"
	assert.ErrorIs(t, parsePlayStorePage([]byte(page), &App{}), ErrAppNotFound)

	// blocks with broken JSON are skipped, and a missing version falls back
	details[140] = nil
	details[145] = []any{[]any{"Oct 31, 2019", []any{1572533883, 121000000}}}
	data, err = json.Marshal([]any{nil, []any{nil, nil, details}})
	assert.NoError(t, err)
	page = "<script>AF_initDataCallback({key: 'ds:2', hash: '1', data:[1,, sideChannel: {}});</script>" +
		"<script>AF_initDataCallback({key: 'ds:5', hash: '2', data:" + string(data) + ", sideChannel: {}});</script>"
	app = App{}
	assert.NoError(t, parsePlayStorePage([]byte(page), &app))
	assert.Equal(t, "Varies with device", app.version)
}

func TestGooglePlayStoreHTTPErrors(t *testing.T) {
	testCases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
		{
			name: "truncated body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				// promise more bytes than are sent so reading the body fails
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte("partial"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			setPlayStoreBaseURL(t, server.URL)

			_, err := GooglePlayStore("com.example.app", "en", "us")
			assert.ErrorIs(t, err, ErrPageLoad)
		})
	}

	t.Run("connection refused", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		setPlayStoreBaseURL(t, server.URL)

		_, err := GooglePlayStore("com.example.app", "en", "us")
		assert.ErrorIs(t, err, ErrPageLoad)
	})
}

func setPlayStoreBaseURL(t *testing.T, baseURL string) {
	t.Helper()
	original := playStoreBaseURL
	playStoreBaseURL = baseURL
	t.Cleanup(func() { playStoreBaseURL = original })
}

func TestGooglePlayStoreNotFound(t *testing.T) {
	_, err := GooglePlayStore("com.katsini.does.not.exist", "en", "us")
	assert.ErrorIs(t, err, ErrAppNotFound)
}

func TestAppleAppStore(t *testing.T) {
	testCases := []struct {
		appID     string
		bundleID  string
		title     string
		url       string
		developer string
	}{
		{
			appID:     "1487875612",
			bundleID:  "com.rostamvpn",
			title:     "RostamVPN - Unlimited Fast VPN",
			url:       "https://apps.apple.com/us/app/rostamvpn-unlimited-fast-vpn/id1487875612?uo=4",
			developer: "Rostam",
		},
		{
			appID:     "1097587096",
			bundleID:  "com.agiletortoise.Diced",
			title:     "Diced - Puzzle Dice Game",
			url:       "https://apps.apple.com/us/app/diced-puzzle-dice-game/id1097587096?uo=4",
			developer: "Agile Tortoise",
		},
		{
			appID:     "1602926022",
			bundleID:  "com.unboxingsolutions.TouchMemory",
			title:     "Sound Matching",
			url:       "https://apps.apple.com/us/app/sound-matching/id1602926022?uo=4",
			developer: "Unboxing Solutions B.V.",
		},
		{
			appID:     "1056101508",
			bundleID:  "com.appgeneration.mytunerpodcastspro",
			title:     "Podcast myTuner - Podcasts App",
			url:       "https://apps.apple.com/us/app/podcast-mytuner-podcasts-app/id1056101508?uo=4",
			developer: "Appgeneration Software",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			app, err := AppleAppStore(tc.appID, tc.bundleID, "us")
			assert.NoError(t, err)
			assert.Equal(t, tc.appID, app.appID)
			assert.Equal(t, tc.bundleID, app.bundleID)
			assert.Equal(t, tc.title, app.title)
			assert.Equal(t, tc.url, app.url)
			assert.Equal(t, tc.developer, app.developer)
		})
	}
}

func TestHuaweiAppGallery(t *testing.T) {
	testCases := []struct {
		appID     string
		bundleID  string
		title     string
		url       string
		developer string
	}{
		{
			appID:     "106093011",
			bundleID:  "com.upapplications.flutter_bird",
			title:     "Flutter Bird",
			url:       "https://appgallery.huawei.com/app/C106093011",
			developer: "UpApplications",
		},
		{
			appID:     "109809367",
			bundleID:  "com.sweet.candy.land.magic.puzzle.huawei",
			title:     "Sweet Candy Land Magic Puzzle",
			url:       "https://appgallery.huawei.com/app/C109809367",
			developer: "Arslan Khalil",
		},
		{
			appID:     "103228579",
			bundleID:  checker5gBundleID,
			title:     checker5gTitle,
			url:       checker5gURL,
			developer: checker5gDeveloper,
		},
		{
			appID:     "107552425",
			bundleID:  "com.colorballsort.watersort.puzzlegame.ballsort.colorsort.puzzle",
			title:     "Color Ball Sorting Brain Puzle",
			url:       "https://appgallery.huawei.com/app/C107552425",
			developer: "BrainStorm Games",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			app, err := HuaweiAppGallery(tc.appID)
			assert.NoError(t, err)
			assert.Equal(t, tc.title, app.title)
			assert.Equal(t, tc.bundleID, app.bundleID)
			assert.Equal(t, tc.url, app.url)
			assert.Equal(t, tc.developer, app.developer)
		})
	}
}

func BenchmarkGooglePlayStore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GooglePlayStore("com.gianlu.timeless", "en", "us")
		assert.NoError(b, err)
	}
}

func BenchmarkAppleAppStore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := AppleAppStore("1602926022", "", "us")
		assert.NoError(b, err)
	}
}

func BenchmarkHuaweiAppGallery(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := HuaweiAppGallery("106093011")
		assert.NoError(b, err)
	}
}
