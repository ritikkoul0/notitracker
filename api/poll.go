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
	// Added a check to prevent unauthorized browser hits if you want
	// if r.Header.Get("Authorization") != "Bearer " + os.Getenv("CRON_SECRET") { ... }

	targetURL := "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"
	
	price, mrp, err := scrapeFlipkart(targetURL)
	if err != nil {
		fmt.Printf("Scrape Error: %v\n", err)
		http.Error(w, "Failed to scrape price", 500)
		return
	}

	fmt.Printf("Price Found: %.2f\n", price)
	sendToDiscord("Samsung 419 L Refrigerator", price, mrp)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"price": %v}`, price)))
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	
	// Crucial: Use a very specific User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("http error: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Updated Regex for 2026 Flipkart structure
	priceReg := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpReg := regexp.MustCompile(`"mrp":\{"value":(\d+)`)

	var price, mrp float64
	pMatch := priceReg.FindStringSubmatch(html)
	mMatch := mrpReg.FindStringSubmatch(html)

	if len(pMatch) > 1 { fmt.Sscanf(pMatch[1], "%f", &price) }
	if len(mMatch) > 1 { fmt.Sscanf(mMatch[1], "%f", &mrp) }

	if price == 0 {
		// Secondary fallback for static price tags
		fallback := regexp.MustCompile(`"price":(\d+)`)
		fMatch := fallback.FindStringSubmatch(html)
		if len(fMatch) > 1 { fmt.Sscanf(fMatch[1], "%f", &price) }
	}

	return price, mrp, nil
}

func sendToDiscord(name string, price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" { return }

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": name,
				"description": fmt.Sprintf("Price updated in **notitracker**"),
				"color": 3447003,
				"fields": []map[string]interface{}{
					{"name": "Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.0f", mrp), "inline": true},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}
