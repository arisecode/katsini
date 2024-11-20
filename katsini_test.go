package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			developer: "AppAzio",
		},
		{
			bundleID:  "pro.flutters.app",
			title:     "Codalingo: Learn Flutter",
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
			title:     "RostamVPN - VPN Fast & Secure",
			url:       "https://apps.apple.com/us/app/rostamvpn-vpn-fast-secure/id1487875612?uo=4",
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
			appID:     "103001419",
			bundleID:  "com.hht.businesscardmaker.huawei",
			title:     "Business Card Maker",
			url:       "https://appgallery.huawei.com/app/C103001419",
			developer: "M/S HAWKS HEAVEN TECHNOLOGIES",
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
