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
	fmt.Println("--- Notitracker: Starting High-Frequency Scrape ---")

	targetURL := "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"

	price, mrp, err := scrapeFlipkart(targetURL)
	if err != nil {
		fmt.Printf("SCRAPE ERROR: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"error","message":"%s"}`, err.Error())
		return
	}

	sendToDiscord("Samsung 419 L Refrigerator", price, mrp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"success","price":%v,"mrp":%v}`, price, mrp)
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)

	// --- ADVANCED SPOOFING HEADERS ---
	// Mimic a click-through from Google Search
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
	req.Header.Set("Sec-Fetch-Site", "cross-site") // Simulates coming from Google
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	
	// OPTIONAL: If still getting 529, paste a fresh cookie from your browser here
	// req.Header.Set("Cookie", "pfetch=1; is_inventory_pfetch=1;")

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

	// Regex for the modern JSON state
	priceReg := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpReg := regexp.MustCompile(`"mrp":\{"value":(\d+)`)

	var price, mrp float64
	pMatch := priceReg.FindStringSubmatch(html)
	mMatch := mrpReg.FindStringSubmatch(html)

	if len(pMatch) > 1 { fmt.Sscanf(pMatch[1], "%f", &price) }
	if len(mMatch) > 1 { fmt.Sscanf(mMatch[1], "%f", &mrp) }

	// Fallback logic
	if price == 0 {
		fallback := regexp.MustCompile(`"price":(\d+)`)
		fMatch := fallback.FindStringSubmatch(html)
		if len(fMatch) > 1 { fmt.Sscanf(fMatch[1], "%f", &price) }
	}

	if price == 0 {
		return 0, 0, fmt.Errorf("parsing failed: price missing (HTML len: %d)", len(html))
	}

	return price, mrp, nil
}

func sendToDiscord(name string, price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" { return }

	discount := ((mrp - price) / mrp) * 100
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": "❄️ Refrigerator Price Drop Alert",
				"description": fmt.Sprintf("**%s**", name),
				"color": 3066993,
				"fields": []map[string]interface{}{
					{"name": "Current Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.0f", mrp), "inline": true},
					{"name": "Discount", "value": fmt.Sprintf("%.1f%%", discount), "inline": true},
				},
				"footer": map[string]string{"text": "Polled via Notitracker • 1m Frequency"},
			},
		},
	}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}
