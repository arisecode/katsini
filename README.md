# Katsini - Get the latest app info from app stores
<br />
<div align="center">
    <img src=".github/assets/logo.png" width="300">
    <br /><br />
</div>
<p>A blazing-fast, lightweight, and user-friendly tool built with <a href="https://golang.org">Go</a> for retrieving app information from major app stores.</p>

[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-blue)](https://go.dev/)
![CI](https://github.com/arisecode/katsini/actions/workflows/build-and-test.yml/badge.svg)
[![Release](https://img.shields.io/github/release/arisecode/katsini.svg?style=flat-square)](https://github.com/arisecode/katsini/releases)
[![codecov](https://codecov.io/gh/arisecode/katsini/branch/master/graph/badge.svg?token=lk4yMGVBOK)](https://codecov.io/gh/arisecode/katsini)
[![GitHub issues](https://img.shields.io/github/issues/arisecode/katsini)](https://github.com/arisecode/katsini/issues)
![GitHub pull requests](https://img.shields.io/github/issues-pr/arisecode/katsini?color=blue&style=flat-square)

## 🍕 Features
- ⚡️ **Blazingly fast** — Lightning-quick app data retrieval powered by Go
- 🛍️ **Multi-store support** — Fetch data from major app stores in one place
- 🪶 **Lightweight** — Minimal resource footprint for efficient operation
- 🐳 **Containerized** — Full Docker support for flexible deployment
- 🔌 **Simple API** — Clean REST endpoints for seamless integration
- 📝 **Structured data** — Consistent JSON output for all app stores
- 🛠️ **Easy setup** — Get started in minutes with straightforward configuration

## 🛍️ Supported App Stores
- ✅ [Google Play Store](https://play.google.com/store)
- ✅ [Apple App Store](https://apps.apple.com)
- ✅ [Huawei AppGallery](https://appgallery.huawei.com)

## 🐳 Quick Start
- **[Docker](https://docs.docker.com/engine/install/)** and **[Docker Compose](https://docs.docker.com/compose/install/)** must be installed on your machine.
- Copy code below and save it as `docker-compose.yml`.
```yaml
services:
  browser:
    image: chromedp/headless-shell:136.0.7052.2
    ports:
      - "9222:9222"
    command: [
      "--no-sandbox",
      "--disable-gpu",
      "--remote-debugging-address=0.0.0.0",
      "--remote-debugging-port=9222",
      "--disable-extensions",
      "--enable-automation",
      "--disable-blink-features=AutomationControlled",
      "--incognito",
      "--user-agent=Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/73.0.3683.103 Safari/537.36"
    ]

  app:
    image: ghcr.io/arisecode/katsini:latest
    environment:
      CHROME_HOST: browser
      CHROME_PORT: 9222
    ports:
      - "8080:8080"
    depends_on:
      - browser
    links:
      - browser
```
- Run `docker-compose up` to start the service.

🎉 That's it! You can now access the service at `http://localhost:8080`

## 📖 Usage

### 🛍️ Google Play Store
#### Example Request:
- **URL:** `http://localhost:8080/playstore`
- **Method:** `GET`
- **Query Parameter:**
    - `bundleId` (**REQUIRED**): The app package name (e.g., `com.mediocre.dirac`).
    - `lang` (optional, defaults to `'**en**'): The two letter language code in which to fetch the app page.
    - `country` (optional, defaults to '**us**'): The two letter country code used to retrieve the applications. Needed when the app is available only in some countries.
```bash
curl http://localhost:8080/playstore?bundleId=com.mediocre.dirac&lang=en&country=us
```
#### Example Response:
```json
{
  "bundleId": "com.mediocre.dirac",
  "developer": "Mediocre",
  "title": "Beyondium",
  "updated": "31-10-2019",
  "url": "https://play.google.com/store/apps/details?id=com.mediocre.dirac&hl=en&gl=us",
  "version": "1.1.5"
}
```

### 🛍️ Apple App Store
#### Example Request:
- **URL:** `http://localhost:8080/appstore`
- **Method:** `GET`
- **Query Parameter:**
    - `appId` or `bundleId` (**REQUIRED**):
      - `appId`: The unique identifier for the application in the Apple App Store. This can be found in the app's store URL after the `/id<APP_ID>` segment.
      - `bundleId`: The app package name (e.g., `com.thinkdivergent`).
    - `country` (optional, defaults to '**us**'): The two letter country code used to retrieve the applications. Needed when the app is available only in some countries.
```bash
curl http://localhost:8080/appstore?appId=1592213654&country=us
```
#### Example Response:
```json
{
  "appId": "1592213654",
  "bundleId": "com.thinkdivergent",
  "developer": "Think Divergent LLC",
  "title": "Think Divergent",
  "updated": "11-02-2023",
  "url": "https://apps.apple.com/us/app/think-divergent/id1592213654?uo=4",
  "version": "2.0.13"
}
```

### 🛍️ Huawei AppGallery
#### Example Request:
- **URL:** `http://localhost:8080/appgallery`
- **Method:** `GET`
- **Query Parameter:**
    - `appId` (**REQUIRED**): The unique identifier for the application in the Huawei AppGallery. This can be found in the app's store URL after the `/app/C<APP_ID>` segment.
```bash
curl http://localhost:8080/appgallery?appId=100102149
```
#### Example Response:
```json
{
  "appId": "100102149",
  "bundleId": "com.radio.fmradio",
  "developer": "RADIOFM",
  "title": "Radio FM",
  "updated": "05-11-2024",
  "url": "https://appgallery.huawei.com/app/C100102149",
  "version": "6.5.6"
}
```

## ⚡ Benchmarks
The benchmarks were run using the following command:
```bash
go test -bench=. -benchtime=1s -benchmem -cpu=1
```
The results of the benchmarks are as follows:
```bash
goos: linux
goarch: amd64
pkg: github.com/arisecode/katsini
cpu: 12th Gen Intel(R) Core(TM) i7-12700
BenchmarkGooglePlayStore  	       2	 811015314 ns/op	 3816876 B/op	   21807 allocs/op
BenchmarkAppleAppStore    	      64	  18702830 ns/op	   95091 B/op	     163 allocs/op
BenchmarkHuaweiAppGallery 	       2	 892189872 ns/op	 2685124 B/op	   15531 allocs/op
```
The benchmark results show the average time taken and how many iterations were run per operation can be done in a second.
- **Note:** The benchmarks were run on a 12th Gen Intel(R) Core(TM) i7-12700 CPU and using a single CPU core.
-  `2` : The number of iterations run per operation.
- `ns/op` : The average time taken for each operation.
- `B/op` : The average number of bytes allocated per operation.
- `allocs/op` : The average number of memory allocations per operation.

## 🔋 Uses
Here are some of the libraries that are used in this project:
- [Chromedp](https://github.com/chromedp/chromedp) - A faster, simpler way to drive browsers in Go.
