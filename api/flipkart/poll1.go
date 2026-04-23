package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Product struct {
	Name string
	URL  string
}

func Handler(w http.ResponseWriter, r *http.Request) {
	// Set the context to 9.5s (Vercel Hobby hard-kills at 10s)
	ctx, cancel := context.WithTimeout(r.Context(), 9500*time.Millisecond)
	defer cancel()

	name := "Havells Rice Cooker"
	url := "https://www.flipkart.com/havells-riso-plus-1-8-l-2-bowl-electric-rice-cooker/p/itm9dc31cc3694d7?pid=ECKGZPNF6PSWGBJN"

	price, mrp, offers, err := streamScrape(ctx, url)
	
	result := "Success"
	if err != nil {
		result = err.Error()
		fmt.Printf("[FAIL] %s: %v\n", name, err)
	} else {
		sendToDiscord(name, price, mrp, offers, url)
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"product":"%s","status":"%s"}`, name, result)
}

func streamScrape(ctx context.Context, url string) (float64, float64, string, error) {
	client := &http.Client{Timeout: 9 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.google.com/")

	resp, err := client.Do(req)
	if err != nil { return 0, 0, "", err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 { return 0, 0, "", fmt.Errorf("HTTP %d", resp.StatusCode) }

	// Use a scanner to find price without downloading the full 600KB page
	scanner := bufio.NewScanner(resp.Body)
	priceReg := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpReg := regexp.MustCompile(`"mrp":\{"value":(\d+)`)
	offerReg := regexp.MustCompile(`"((\d+%\s+Off\s+on\s+[A-Z\s]+Bank)|(Bank\s+Offer\s+₹\d+))"`)

	var price, mrp float64
	var offers string

	for scanner.Scan() {
		line := scanner.Text()
		
		if price == 0 {
			if m := priceReg.FindStringSubmatch(line); len(m) > 1 { fmt.Sscanf(m[1], "%f", &price) }
		}
		if mrp == 0 {
			if m := mrpReg.FindStringSubmatch(line); len(m) > 1 { fmt.Sscanf(m[1], "%f", &mrp) }
		}
		if offers == "" {
			if m := offerReg.FindStringSubmatch(line); len(m) > 1 { offers = strings.Trim(m[1], `"`) }
		}

		// Optimization: If we found both prices, stop reading the body immediately
		if price > 0 && mrp > 0 { break }
	}

	if price == 0 { return 0, 0, "", fmt.Errorf("Price missing in top 150KB") }
	if mrp == 0 { mrp = price }
	if offers == "" { offers = "No offers found" }

	return price, mrp, offers, nil
}

func sendToDiscord(name string, price, mrp float64, offers, link string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" { return }

	discount := ((mrp - price) / mrp) * 100
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": name, "url": link, "color": 3066993,
				"fields": []map[string]interface{}{
					{"name": "Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "Discount", "value": fmt.Sprintf("%.0f%%", discount), "inline": true},
					{"name": "Bank", "value": offers, "inline": false},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}
