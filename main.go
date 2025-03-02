package main

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	client *http.Client
	once   sync.Once
)

func main() {
	e := echo.New()
	e.GET("/", handleImageProxyRequest)
	_ = e.Start(":8888")
}

func getHTTPClient() *http.Client {
	once.Do(func() {
		client = &http.Client{
			Timeout: time.Second * 30,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     time.Second * 90,
			},
		}
	})
	return client
}

func handleImageProxyRequest(c echo.Context) error {
	targetURL := c.QueryParam("url")
	if targetURL == "" {
		return c.String(http.StatusBadRequest, "url is empty")
	}
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return c.String(http.StatusBadRequest, "url is invalid")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), time.Second*30)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	setHeader(req, parsedURL, c.RealIP())
	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return handleFallback(c, targetURL)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "image") {
		return c.String(http.StatusBadRequest, "content-type is invalid")
	}
	c.Response().Header().Set("Content-Type", contentType)
	c.Response().WriteHeader(resp.StatusCode)
	_, err = io.Copy(c.Response().Writer, resp.Body)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return nil
}

// handleFallback use wsrv.nl as fallback
func handleFallback(c echo.Context, originalURL string) error {
	imageURL := fmt.Sprintf("https://images.weserv.nl/?url=%s", originalURL)
	ctx, cancel := context.WithTimeout(c.Request().Context(), time.Second*30)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	c.Response().Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Response().WriteHeader(resp.StatusCode)
	_, err = io.Copy(c.Response().Writer, resp.Body)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return nil
}

func setHeader(req *http.Request, parsedURL *url.URL, ip string) {
	hostname := parsedURL.Hostname()
	req.Header.Set("Referer", fmt.Sprintf("https://%s", hostname))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("X-Forwarded-For", ip)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host))
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
}
