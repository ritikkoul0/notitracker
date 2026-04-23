package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	// The specific URL for the 419L Samsung Refrigerator
	url := "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"
	
	price, mrp, err := scrapeFlipkart(url)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		http.Error(w, fmt.Sprintf("Scraping failed: %v", err), 500)
		return
	}

	sendToDiscord("Samsung 419 L Double Door", price, mrp)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","price":%v,"mrp":%v}`, price, mrp)
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	
	// Mimic a modern desktop browser to bypass basic bot filters
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Modern Flipkart JSON-LD and Preloaded State patterns
	// Pattern 1: targeting the "value" inside the sellingPrice/mrp objects
	priceRegex := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpRegex := regexp.MustCompile(`"mrp":\{"value":(\d+)`)

	var price, mrp float64
	pMatches := priceRegex.FindStringSubmatch(html)
	mMatches := mrpRegex.FindStringSubmatch(html)

	if len(pMatches) > 1 {
		fmt.Sscanf(pMatches[1], "%f", &price)
	}
	if len(mMatches) > 1 {
		fmt.Sscanf(mMatches[1], "%f", &mrp)
	}

	// Pattern 2: Fallback to the standard SEO price field if Pattern 1 fails
	if price == 0 {
		fallbackRegex := regexp.MustCompile(`"price":(\d+)`)
		fMatches := fallbackRegex.FindStringSubmatch(html)
		if len(fMatches) > 1 {
			fmt.Sscanf(fMatches[1], "%f", &price)
		}
	}

	if price == 0 {
		return 0, 0, fmt.Errorf("could not find price data in page source")
	}

	return price, mrp, nil
}

func sendToDiscord(name string, price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": fmt.Sprintf("📊 Price Update: %s", name),
				"color": 5814783, // Blurple
				"fields": []map[string]interface{}{
					{"name": "Current Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.0f", mrp), "inline": true},
					{"name": "Discount", "value": fmt.Sprintf("%.0f%%", ((mrp-price)/mrp)*100), "inline": true},
				},
				"footer": map[string]string{"text": "Ritik's Money Manager Poller"},
			},
		},
	}
	body, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
}
