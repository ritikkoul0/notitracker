package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type ScrapeResult struct {
	Price   string `json:"price"`
	Discord string `json:"discord"`
}

// Global client with aggressive timeouts
var fastClient = &http.Client{
	Timeout: 5 * time.Second, 
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	},
}

func Handler(w http.ResponseWriter, r *http.Request) {
	// Hobby limit is 10s. We MUST finish by 9s.
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Second)
	defer cancel()

	productList := []Product{
		{"Havells Rice Cooker", "https://www.flipkart.com/havells-riso-plus-1-8-l-2-bowl-electric-rice-cooker/p/itm9dc31cc3694d7?pid=ECKGZPNF6PSWGBJN"},
		{"LG Microwave Oven", "https://www.flipkart.com/lg-20-l-i-wave-technology-indian-cuisine-auto-cook-menu-steam-clean-anti-bacterial-cavity-health-plus-menu-grill-microwave-oven/p/itmdz5v2y3ckyu9y?pid=MRCDZ5VFHGD7F7UV"},
	}

	results := make(map[string]ScrapeResult)

	for _, p := range productList {
		// Check if we still have time left in our 9s budget
		if ctx.Err() != nil {
			results[p.Name] = ScrapeResult{Price: "Timed Out", Discord: "N/A"}
			continue
		}

		price, mrp, offers, err := scrapeFlipkart(ctx, p.URL)
		if err != nil {
			results[p.Name] = ScrapeResult{Price: err.Error(), Discord: "N/A"}
			continue
		}

		dStatus := "Sent"
		if dErr := sendToDiscord(p.Name, price, mrp, offers, p.URL); dErr != nil {
			dStatus = dErr.Error()
		}
		
		results[p.Name] = ScrapeResult{Price: fmt.Sprintf("₹%.0f", price), Discord: dStatus}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func scrapeFlipkart(ctx context.Context, url string) (float64, float64, string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.google.com/")

	resp, err := fastClient.Do(req)
	if err != nil { return 0, 0, "", err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 { return 0, 0, "", fmt.Errorf("HTTP %d", resp.StatusCode) }

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	priceReg := regexp.MustCompile(`"sellingPrice":\{"value":(\d+)`)
	mrpReg := regexp.MustCompile(`"mrp":\{"value":(\d+)`)
	
	var price, mrp float64
	if m := priceReg.FindStringSubmatch(html); len(m) > 1 { fmt.Sscanf(m[1], "%f", &price) }
	if m := mrpReg.FindStringSubmatch(html); len(m) > 1 { fmt.Sscanf(m[1], "%f", &mrp) }

	offerRegex := regexp.MustCompile(`"((\d+%\s+Off\s+on\s+[A-Z\s]+Bank)|(Bank\s+Offer\s+₹\d+))"`)
	match := offerRegex.FindStringSubmatch(html)
	offers := "No offers"
	if len(match) > 1 { offers = strings.Trim(match[1], `"`) }

	if price == 0 { return 0, 0, "", fmt.Errorf("Parse Fail") }
	if mrp == 0 { mrp = price }
	return price, mrp, offers, nil
}

func sendToDiscord(name string, price, mrp float64, offers, link string) error {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" { return nil }

	discount := 0.0
	if mrp > 0 { discount = ((mrp - price) / mrp) * 100 }

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": name, "url": link, "color": 3066993,
				"fields": []map[string]interface{}{
					{"name": "Price", "value": fmt.Sprintf("₹%.0f", price), "inline": true},
					{"name": "Disc", "value": fmt.Sprintf("%.0f%%", discount), "inline": true},
					{"name": "Bank", "value": offers, "inline": false},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	// Dedicated timeout for Discord
	dCtx, dCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer dCancel()
	
	req, _ := http.NewRequestWithContext(dCtx, "POST", webhookURL, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := fastClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	return nil
}
