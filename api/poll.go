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

type Product struct {
	Name string
	URL  string
}

// Handler is the entry point for Vercel.
func Handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("--- Notitracker: Starting Batch Scrape ---")

	// 1. ADD YOUR LINKS HERE
	productList := []Product{
		{
			Name: "Samsung 419 L Refrigerator",
			URL:  "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM",
		},
		{
			Name: "Samsung 653 L Side-by-Side",
			URL:  "https://www.flipkart.com/samsung-653-l-frost-free-side-by-side-3-star-refrigerator-with-ai-and-wifi-convertible-5-in-1-digital-inverter/p/itm7e086361f0c13?pid=RFRGRZSQ4GQUHFE2",
		},
		// Add more products as needed
	}

	summary := make(map[string]interface{})

	for _, p := range productList {
		price, mrp, err := scrapeFlipkart(p.URL)
		if err != nil {
			fmt.Printf("SCRAPE ERROR [%s]: %v\n", p.Name, err)
			summary[p.Name] = fmt.Sprintf("Error: %v", err)
			continue
		}

		fmt.Printf("SUCCESS [%s]: Price ₹%.0f\n", p.Name, price)
		sendToDiscord(p.Name, price, mrp)
		summary[p.Name] = price
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(summary)
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)

	// Advanced Browser Spoofing
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-IN,en;q=0.9,hi-IN;q=0.8,hi;q=0.7,en-GB;q=0.6,en-US;q=0.5")
	req.Header.Set("Referer", "https://www.google.com/")
	req.Header.Set("Origin", "https://www.flipkart.com")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="124", "Chromium";v="124", "Not-A.Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("Flipkart Blocked (Status %d)", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	var price, mrp float64

	// --- 1. EXTRACT SELLING PRICE ---
	pricePatterns := []string{
		`"sellingPrice":\{"value":(\d+)`,
		`"price":(\d+)`,
		`"decimalValue":"(\d+)"`,
	}
	for _, pattern := range pricePatterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(html)
		if len(match) > 1 {
			fmt.Sscanf(match[1], "%f", &price)
			if price > 0 { break }
		}
	}

	// --- 2. EXTRACT MRP ---
	mrpPatterns := []string{
		`"mrp":\{"value":(\d+)`,
		`"strikeOffPrice":\{"value":(\d+)`,
		`"listPrice":\{"value":(\d+)`,
		`"mrp":(\d+)`,
	}
	for _, pattern := range mrpPatterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(html)
		if len(match) > 1 {
			fmt.Sscanf(match[1], "%f", &mrp)
			if mrp > 0 { break }
		}
	}

	if mrp == 0 { mrp = price }
	if price == 0 {
		return 0, 0, fmt.Errorf("parsing failed: price missing")
	}

	return price, mrp, nil
}

func sendToDiscord(name string, price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" { return }

	discount := 0.0
	if mrp > 0 {
		discount = ((mrp - price) / mrp) * 100
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": "📊 Notitracker: " + name,
				"color": 3066993,
				"fields": []map[string]interface{}{
					{"name": "Current Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.0f", mrp), "inline": true},
					{"name": "Discount", "value": fmt.Sprintf("%.1f%%", discount), "inline": true},
				},
				"footer": map[string]string{"text": "Batch Poller • Notitracker"},
			},
		},
	}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}
