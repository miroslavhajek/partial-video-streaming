package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/errgroup"
)

func main() {
	mainServer := &http.Server{
		Addr:    ":8000",
		Handler: mainRouter(),
	}

	fileServer := &http.Server{
		Addr:    ":8001",
		Handler: videoRouter(),
	}

	g := errgroup.Group{}
	g.Go(func() error {
		return mainServer.ListenAndServe()
	})
	g.Go(func() error {
		return fileServer.ListenAndServe()
	})

	if err := g.Wait(); err != nil {
		panic(err)
	}
}

func videoRouter() http.Handler {
	router := echo.New()
	router.GET("/video", func(c echo.Context) error {
		return c.File("video.mp4")
	})

	return router
}

func mainRouter() http.Handler {
	router := echo.New()
	router.GET("/video", video)
	router.GET("", indexPage)

	return router
}

func indexPage(c echo.Context) error {
	return c.HTML(http.StatusOK, `<html>
		<head></head>
		<body>
			<video src="/video"
				width="640"
				height="480"
				controls
				autoplay
				crossorigin="anonymous"
				playsinline=""
				webkit-playsinline=""
			>
		</video>
		</body>
		</html>`,
	)
}

const kB = 1024
const firstChunkSize = 100 * kB
const chunkSize = 2 * kB * kB

func video(c echo.Context) error {
	requestedRange := c.Request().Header.Get("Range")

	chunkStart := 0
	chunkEnd := firstChunkSize

	headerLength := len(requestedRange)
	if headerLength > 0 {
		chunkStart, _ = strconv.Atoi(requestedRange[6 : headerLength-1])
		chunkEnd = chunkStart + chunkSize
	}

	log.Printf("Requested: %s, size: %d-%d", requestedRange, chunkStart, chunkEnd)

	// Load video from virtual server
	client := new(http.Client)
	resp, err := client.Get("http://localhost:8001/video")
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Read video by origin Range
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return err
	}
	videoBytes := buf.Bytes()
	totalBytes := len(videoBytes)

	// Overflow check
	if chunkEnd > totalBytes {
		chunkEnd = totalBytes
	}

	// Trim video
	out := videoBytes[chunkStart:chunkEnd]

	// Send chunk
	contentRangeHeader := fmt.Sprintf("bytes %d-%d/%d", chunkStart, chunkEnd, totalBytes)

	log.Printf("Response: %s", contentRangeHeader)

	headers := c.Response().Header()
	headers.Set(echo.HeaderContentLength, resp.Header.Get("Content-Length"))
	headers.Set("Content-Range", contentRangeHeader)

	return c.Blob(http.StatusPartialContent, "video/mp4", out)
}
