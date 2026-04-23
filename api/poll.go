package handler // This MUST be handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
)

// Handler is the entry point for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Force a log that WILL show up in Vercel Dashboard
	fmt.Println("CRON JOB TRIGGERED: Starting Flipkart Scrape...")

	targetURL := "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"
	
	price, mrp, err := scrapeFlipkart(targetURL)
	if err != nil {
		fmt.Printf("SCRAPE ERROR: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error: " + err.Error()))
		return
	}

	// 2. Log the findings
	fmt.Printf("SUCCESS: Name: Samsung 419L, Price: %.2f, MRP: %.2f\n", price, mrp)

	sendToDiscord("Samsung 419 L Refrigerator", price, mrp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"completed","price":%v}`, price)
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	
	// Use a standard modern User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Broad regex to catch the prices in the JSON state
	priceReg := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpReg := regexp.MustCompile(`"mrp":\{"value":(\d+)`)

	var price, mrp float64
	pMatch := priceReg.FindStringSubmatch(html)
	mMatch := mrpReg.FindStringSubmatch(html)

	if len(pMatch) > 1 { fmt.Sscanf(pMatch[1], "%f", &price) }
	if len(mMatch) > 1 { fmt.Sscanf(mMatch[1], "%f", &mrp) }

	if price == 0 {
		return 0, 0, fmt.Errorf("price not found in HTML (Length: %d)", len(html))
	}

	return price, mrp, nil
}

func sendToDiscord(name string, price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		fmt.Println("DISCORD ERROR: Webhook URL is missing from Env")
		return
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": name,
				"color": 3447003,
				"fields": []map[string]interface{}{
					{"name": "Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "MRP", "value": fmt.Sprintf("₹%.0f", mrp), "inline": true},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	_, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("DISCORD ERROR: %v\n", err)
	}
}
