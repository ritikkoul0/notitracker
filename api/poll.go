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
	url := "https://www.flipkart.com/samsung-419-l-frost-free-double-door-3-star-convertible-refrigerator-5-in-1-digital-inverter-wifi-enabled-bespoke-ai/p/itm8e086361f0c13?pid=RFRH3T3HQQEH6QZM"
	
	price, mrp, err := scrapeFlipkart(url)
	if err != nil {
		http.Error(w, "Scraping failed", 500)
		return
	}

	sendToDiscord("Samsung 419 L Double Door", price, mrp)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","price":%v}`, price)
}

func scrapeFlipkart(url string) (float64, float64, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	priceRegex := regexp.MustCompile(`"price":"(\d+)"`)
	mrpRegex := regexp.MustCompile(`"mrp":(\d+)`)

	var price, mrp float64
	pMatches := priceRegex.FindStringSubmatch(html)
	mMatches := mrpRegex.FindStringSubmatch(html)

	if len(pMatches) > 1 { fmt.Sscanf(pMatches[1], "%f", &price) }
	if len(mMatches) > 1 { fmt.Sscanf(mMatches[1], "%f", &mrp) }

	return price, mrp, nil
}

func sendToDiscord(name string, price, mrp float64) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
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
	body, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
}
