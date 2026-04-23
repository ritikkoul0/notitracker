package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Product struct {
	Name string
	URL  string
}

// Shared transport for connection pooling
var sharedTransport = &http.Transport{
	MaxIdleConns:        20,
	IdleConnTimeout:     90 * time.Second,
	DisableKeepAlives:   false,
}

func Handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("--- Notitracker: Starting Optimized Parallel Scrape ---")

	productList := []Product{
		{"Havells Rice Cooker", "https://www.flipkart.com/havells-riso-plus-1-8-l-2-bowl-electric-rice-cooker/p/itm9dc31cc3694d7?pid=ECKGZPNF6PSWGBJN"},
		{"LG Microwave Oven", "https://www.flipkart.com/lg-20-l-i-wave-technology-indian-cuisine-auto-cook-menu-steam-clean-anti-bacterial-cavity-health-plus-menu-grill-microwave-oven/p/itmdz5v2y3ckyu9y?pid=MRCDZ5VFHGD7F7UV"},
		{"Atomberg Renesa Fan", "https://www.flipkart.com/atomberg-renesa-halo-smart-voice-controlled-high-air-flow-low-noise-led-speed-indicator-3-year-warranty-bldc-motor-remote-1200-mm-ceiling-fan/p/itm39f1a608fb2aa?pid=FANH9H58ZJ3T5UJM"},
		{"Samsung Bespoke Fridge", "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"},
		{"Whirlpool Chimney", "https://www.flipkart.com/whirlpool-cgbf-pro-903-hac-bk-hood-auto-clean-curved-glass-90-cm-11-years-motor-warranty-heat-autoclean-gesture-control-baffle-filter-powerful-suction-low-noise-wall-mounted-black-1500-cmh-chimney/p/itm12e07fcfaaef4?pid=CHYGT59WPSPNCFG6"},
		{"AO Smith Water Purifier", "https://www.flipkart.com/ao-smith-z2-5-l-ro-water-purifier-6-stages-purification-digital-display-under-sink-placement-complimentary-faucet-suitable-all-borewell-cancer-municipality/p/itm67fa720667ccb?pid=WAPF943KSMEKKRH9"},
		{"Prestige Hob", "https://www.flipkart.com/prestige-svachh-efficia-03-ai-8mm-thick-superior-toughened-glass-cast-iron-pan-support-glass-automatic-hob/p/itm1fdeb478eafc8?pid=GSTH5G9FFTPGMZGF"},
		{"Whirlpool Washing Machine", "https://www.flipkart.com/whirlpool-7-kg-magic-clean-5-star-fully-automatic-top-load-washing-machine-grey/p/itm50fdb8ca1e478?pid=WMNGDSUXZS5BWH7H"},
		{"Sony Bravia 65 inch TV", "https://www.flipkart.com/sony-bravia-2-ii-163-9-cm-65-inch-ultra-hd-4k-led-smart-google-tv-2025/p/itm79726a02d6955?pid=TVSHBYPVYRDZQG4B"},
		{"Bajaj Steam Iron", "https://www.flipkart.com/bajaj-mx-45-steam-iron-2000-w/p/itm7f635ea3b19c1?pid=IRNHFYJGBTGPY5YP"},
		{"Kent Egg Boiler", "https://www.flipkart.com/kent-super-egg-boiler-stainless-steel-body-heating-plate-automatic-turn-off-116069-cooker/p/itme6a3998ac7b46?pid=EGGFW2WJV4FNYFMM"},
		{"Atomberg Exhaust Fan", "https://www.flipkart.com/atomberg-studio-exhaust-6-inch6-8wlow-noiseeasy-cleaninstallation-round-cut-153mm-150-mm-fan/p/itmf71412247ee19?pid=EXFFQGVDGYPBF9UD"},
		{"Philips Air Fryer", "https://www.flipkart.com/philips-na120-00-uses-up-90-less-fat-1500w-rapid-air-technology-fryer/p/itmaf10e1713a251?pid=AFRHY864CKYUN2YH"},
		{"Bosch Mixer Grinder", "https://www.flipkart.com/bosch-pro-1000-w-mixer-grinder/p/itm95c51f4a5bc94?pid=MIXHGPUMUGFECGNM"},
		{"AO Smith Geyser", "https://www.flipkart.com/ao-smith-hse-shs-025-25-l-storage-water-geyser-2kw-vertical-designed-high-rise-buildings-8-bar-pressure-rating-longer-life-hard-conditions-blue-diamond-glass-lined-tank-bee-5-star-superior-energy-efficiency-power-savings/p/itm1a81ed23a8b92?pid=WGYGGKJXAZHNZVGR"},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]string)
	
	// SEMAPHORE: Limit to 3 concurrent requests to prevent Vercel/Flipkart choking
	sem := make(chan struct{}, 3)

	for _, p := range productList {
		wg.Add(1)
		go func(prod Product) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire token
			defer func() { <-sem }() // Release token

			// Reduced timeout to ensure we finish under Vercel's 10s total limit
			price, mrp, offers, err := scrapeFlipkart(prod.URL)
			
			mu.Lock()
			if err != nil {
				results[prod.Name] = err.Error()
			} else {
				results[prod.Name] = fmt.Sprintf("₹%.0f", price)
				sendToDiscord(prod.Name, price, mrp, offers, prod.URL)
			}
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func scrapeFlipkart(url string) (float64, float64, string, error) {
	// 5 second timeout per request to keep total time low
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: sharedTransport,
	}
	
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, "", fmt.Errorf("Flipkart Status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	var price, mrp float64
	priceRegex := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpRegex := regexp.MustCompile(`"mrp":\{"value":(\d+)`)
	
	if m := priceRegex.FindStringSubmatch(html); len(m) > 1 { fmt.Sscanf(m[1], "%f", &price) }
	if m := mrpRegex.FindStringSubmatch(html); len(m) > 1 { fmt.Sscanf(m[1], "%f", &mrp) }

	offerRegex := regexp.MustCompile(`"((\d+%\s+Off\s+on\s+[A-Z\s]+Bank)|(Bank\s+Offer\s+₹\d+))"`)
	matches := offerRegex.FindAllStringSubmatch(html, 1)
	offerSummary := "No bank offers"
	if len(matches) > 0 { offerSummary = strings.Trim(matches[0][1], `"`) }

	if mrp == 0 { mrp = price }
	return price, mrp, offerSummary, nil
}

func sendToDiscord(name string, price, mrp float64, offers, link string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" { return }

	discount := 0.0
	if mrp > 0 { discount = ((mrp - price) / mrp) * 100 }

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": name,
				"url":   link,
				"color": 3447003,
				"fields": []map[string]interface{}{
					{"name": "Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "Discount", "value": fmt.Sprintf("%.0f%%", discount), "inline": true},
					{"name": "Bank Offer", "value": offers, "inline": false},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}
