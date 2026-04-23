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

// Handler is the entry point for Vercel.
// Function name must be 'Handler' and package must be 'handler'.
func Handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Scrape initiated...")

	// Target URL for the Samsung 419L
	targetURL := "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"

	price, mrp, err := scrapeFlipkart(targetURL)
	if err != nil {
		fmt.Printf("Scrape Failed: %v\n", err)
		// Return 200 even on error so the Cron service doesn't think the server is down,
		// but log the error to the response.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"error","message":"%s"}`, err.Error())
		return
	}

	fmt.Printf("Found Price: %.0f\n", price)
	sendToDiscord("Samsung 419 L Refrigerator", price, mrp)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","price":%v,"mrp":%v}`, price, mrp)
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)

	// Rotating common User-Agents helps prevent 403 blocks
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("Flipkart returned status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Regex targeting the JSON state object in Flipkart's HTML
	priceReg := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpReg := regexp.MustCompile(`"mrp":\{"value":(\d+)`)

	var price, mrp float64
	pMatch := priceReg.FindStringSubmatch(html)
	mMatch := mrpReg.FindStringSubmatch(html)

	if len(pMatch) > 1 {
		fmt.Sscanf(pMatch[1], "%f", &price)
	}
	if len(mMatch) > 1 {
		fmt.Sscanf(mMatch[1], "%f", &mrp)
	}

	if price == 0 {
		// Fallback for different JSON structure
		fallback := regexp.MustCompile(`"price":(\d+)`)
		fMatch := fallback.FindStringSubmatch(html)
		if len(fMatch) > 1 {
			fmt.Sscanf(fMatch[1], "%f", &price)
		}
	}

	if price == 0 {
		return 0, 0, fmt.Errorf("could not extract price from HTML")
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
				"title": name,
				"color": 3447003,
				"fields": []map[string]interface{}{
					{"name": "Current Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.0f", mrp), "inline": true},
				},
				"footer": map[string]string{"text": "Hourly Update via Notitracker"},
			},
		},
	}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}
