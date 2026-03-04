package handlers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"image-resizer/config"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/gofiber/fiber/v2"
)

type ImageHandler struct {
	cfg *config.Config
}

type ResizeBatchJob struct {
	Key      string   `json:"key"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Formats  []string `json:"format"`
	Quality  int      `json:"quality"`
	Lossless bool     `json:"lossless"`
}

func NewImageHandler(cfg *config.Config) *ImageHandler {
	vips.Startup(nil)
	return &ImageHandler{cfg: cfg}
}

func (h *ImageHandler) Shutdown() {
	vips.Shutdown()
}

func (h *ImageHandler) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "healthy",
		"version": vips.Version,
	})
}

// getEffort determines the encoding effort based on the number of pixels.
// Higher effort provides better compression but is slower.
// We use lower effort for larger images to save compute time.
func getEffort(pixels int) int {
	switch {
	case pixels < 500_000: // < 0.5 MP
		return 6
	case pixels < 1_000_000: // < 1 MP
		return 5
	case pixels < 2_000_000: // < 2 MP
		return 4
	case pixels < 4_000_000: // < 4 MP
		return 3
	default: // >= 4 MP
		return 2
	}
}

func (h *ImageHandler) Resize(c *fiber.Ctx) error {
	var err error
	var originalBaseName string
	var tempPath string

	// Create a temp file to store the image for processing
	// We use a temp file to avoid holding the entire image in memory
	// and to allow efficient random access by libvips.
	tmpFile, err := os.CreateTemp("", "image-resizer-*.img")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create temp file", "code": 1016})
	}
	tempPath = tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tempPath)

	if c.Method() == fiber.MethodPost {
		// Handle file upload from multipart form
		fileHeader, err := c.FormFile("image")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Image file 'image' is required for POST requests", "code": 1009})
		}
		originalFilename := fileHeader.Filename
		originalBaseName = strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

		if err := c.SaveFile(fileHeader, tempPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save uploaded image", "code": 1010})
		}
	} else {
		// Handle URL download from query param
		sourceURL := c.Query("url")
		if sourceURL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL parameter required for GET requests", "code": 1001})
		}

		parsedURL, err := url.ParseRequestURI(sourceURL)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL format", "code": 1002})
		}
		pathParts := strings.Split(parsedURL.Path, "/")
		originalFilename := pathParts[len(pathParts)-1]
		originalBaseName = strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

		if len(h.cfg.Image.AllowedHosts) > 0 {
			isAllowed := false
			for _, allowedHost := range h.cfg.Image.AllowedHosts {
				if strings.HasSuffix(parsedURL.Host, allowedHost) {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Host not allowed", "code": 1003})
			}
		}

		client := http.Client{Timeout: h.cfg.Image.Timeout}
		resp, err := client.Get(sourceURL)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Failed to download image", "code": 1004})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("Failed to download image: status %d", resp.StatusCode), "code": 1004})
		}

		// Write download to temp file
		f, err := os.Create(tempPath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open temp file", "code": 1016})
		}
		_, err = io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Failed to save downloaded image", "code": 1004})
		}
	}

	// Load image to check validity and get dimensions
	img, err := vips.NewImageFromFile(tempPath)
	if err != nil {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "Failed to decode image", "code": 1005})
	}
	defer img.Close()

	// 6. Parse tasks
	var jobs []ResizeBatchJob
	jobsStr := c.FormValue("tasks")
	if jobsStr == "" {
		jobsStr = c.Query("tasks")
	}

	isBatch := false
	if jobsStr != "" {
		if err := json.Unmarshal([]byte(jobsStr), &jobs); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid tasks JSON", "code": 1011})
		}
		isBatch = true
	} else {
		// Legacy single task mode
		width, _ := strconv.Atoi(c.Query("width"))
		height, _ := strconv.Atoi(c.Query("height"))
		format := c.Query("format", "jpg")
		quality, _ := strconv.Atoi(c.Query("quality"))
		lossless, _ := strconv.ParseBool(c.Query("lossless"))

		jobs = append(jobs, ResizeBatchJob{
			Width:    width,
			Height:   height,
			Formats:  []string{format},
			Quality:  quality,
			Lossless: lossless,
		})
	}

	// 7. Process
	if !isBatch {
		// Single image response
		task := ResizeTask{
			Width:    jobs[0].Width,
			Height:   jobs[0].Height,
			Format:   jobs[0].Formats[0],
			Quality:  jobs[0].Quality,
			Lossless: jobs[0].Lossless,
		}
		buf, contentType, err := h.processTask(img, task)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "code": 1007})
		}
		c.Set("Content-Type", contentType)
		return c.Send(buf)
	}

	// Batch response
	archiveType := c.FormValue("archive")
	if archiveType == "" {
		archiveType = c.Query("archive")
	}
	if archiveType == "" || archiveType == "tar.cz" {
		archiveType = "tar.gz"
	}

	var addFile func(name string, data []byte) error
	var closeArchive func() error

	if archiveType == "zip" {
		c.Set("Content-Type", "application/zip")
		c.Set("Content-Disposition", "attachment; filename=\"images.zip\"")
		zw := zip.NewWriter(c.Response().BodyWriter())
		addFile = func(name string, data []byte) error {
			f, err := zw.Create(name)
			if err != nil {
				return err
			}
			_, err = f.Write(data)
			return err
		}
		closeArchive = zw.Close
	} else if archiveType == "tar" {
		c.Set("Content-Type", "application/x-tar")
		c.Set("Content-Disposition", "attachment; filename=\"images.tar\"")
		tw := tar.NewWriter(c.Response().BodyWriter())
		addFile = func(name string, data []byte) error {
			hdr := &tar.Header{
				Name:    name,
				Mode:    0644,
				Size:    int64(len(data)),
				ModTime: time.Now(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			_, err := tw.Write(data)
			return err
		}
		closeArchive = tw.Close
	} else {
		// Default: tar.gz
		c.Set("Content-Type", "application/x-tar+gzip")
		c.Set("Content-Disposition", "attachment; filename=\"images.tar.gz\"")
		gw := gzip.NewWriter(c.Response().BodyWriter())
		tw := tar.NewWriter(gw)
		addFile = func(name string, data []byte) error {
			hdr := &tar.Header{
				Name:    name,
				Mode:    0644,
				Size:    int64(len(data)),
				ModTime: time.Now(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			_, err := tw.Write(data)
			return err
		}
		closeArchive = func() error {
			tw.Close()
			return gw.Close()
		}
	}
	defer closeArchive()

	type ImageManifest struct {
		Filename string `json:"filename"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Format   string `json:"format"`
		Key      string `json:"key"`
		Base     string `json:"base"`
	}
	var manifest []ImageManifest

	for i, job := range jobs {
		// Create new image from buffer for this task to allow independent shrink-on-load
		taskImg, err := vips.NewImageFromFile(tempPath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load image for task", "code": 1012})
		}

		// Resize once per job
		originalWidth := taskImg.Width()
		originalHeight := taskImg.Height()
		targetWidth := job.Width
		targetHeight := job.Height

		if targetWidth == 0 && targetHeight == 0 {
			targetWidth = originalWidth
			targetHeight = originalHeight
		} else if targetWidth > 0 && targetHeight == 0 {
			scale := float64(targetWidth) / float64(originalWidth)
			targetHeight = int(float64(originalHeight) * scale)
		} else if targetHeight > 0 && targetWidth == 0 {
			scale := float64(targetHeight) / float64(originalHeight)
			targetWidth = int(float64(originalWidth) * scale)
		}

		// Enforce global maximum dimensions from environment
		if mwStr := os.Getenv("MAX_WIDTH"); mwStr != "" {
			if mw, err := strconv.Atoi(mwStr); err == nil && targetWidth > mw {
				taskImg.Close()
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("width %d exceeds maximum allowed %d", targetWidth, mw)})
			}
		}
		if mhStr := os.Getenv("MAX_HEIGHT"); mhStr != "" {
			if mh, err := strconv.Atoi(mhStr); err == nil && targetHeight > mh {
				taskImg.Close()
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("height %d exceeds maximum allowed %d", targetHeight, mh)})
			}
		}

		if err := taskImg.Thumbnail(targetWidth, targetHeight, vips.InterestingNone); err != nil {
			taskImg.Close()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to resize image", "code": 1006})
		}

		finalWidth := taskImg.Width()
		finalHeight := taskImg.Height()
		pixels := finalWidth * finalHeight
		effort := getEffort(pixels)

		// Export for each format
		for _, fmtStr := range job.Formats {
			buf, contentType, err := h.exportImage(taskImg, fmtStr, job.Quality, job.Lossless, effort, finalWidth, finalHeight)
			if err != nil {
				taskImg.Close()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "code": 1013})
			}

			ext := "jpg"
			switch contentType {
			case "image/png":
				ext = "png"
			case "image/webp":
				ext = "webp"
			case "image/avif":
				ext = "avif"
			case "image/jxl":
				ext = "jxl"
			}

			var filename string
			if job.Key != "" {
				filename = fmt.Sprintf("%s_%s.%s", originalBaseName, job.Key, ext)
				if originalBaseName == "" {
					filename = fmt.Sprintf("image_%s.%s", job.Key, ext)
				}
			} else {
				if originalBaseName == "" {
					filename = fmt.Sprintf("image_%d.%s", i+1, ext)
				} else {
					filename = fmt.Sprintf("%s_%d.%s", originalBaseName, i+1, ext)
				}
			}

			if err := addFile(filename, buf); err != nil {
				taskImg.Close()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to write to archive", "code": 1015})
			}

			manifest = append(manifest, ImageManifest{
				Filename: filename,
				Width:    finalWidth,
				Height:   finalHeight,
				Format:   ext,
				Key:      job.Key,
				Base:     originalBaseName,
			})
		}
		taskImg.Close()
	}

	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		addFile("manifest.json", manifestJSON)
	}

	return nil
}

// Helper struct for single task processing (legacy)
type ResizeTask struct {
	Width    int
	Height   int
	Format   string
	Quality  int
	Lossless bool
}

func (h *ImageHandler) exportImage(img *vips.ImageRef, format string, quality int, lossless bool, effort int, width, height int) ([]byte, string, error) {
	format = strings.ToLower(format)
	if format == "" {
		format = "jpg"
	}

	// Scale quality based on format to match JPEG visual fidelity
	if quality > 0 {
		switch format {
		case "avif":
			quality = int(float64(quality) * 0.8)
		case "jxl":
			quality = int(float64(quality) * 0.75)
		}
		if quality < 1 {
			quality = 1
		}
	}

	var buf []byte
	var contentType string
	var err error

	switch format {
	case "avif":
		if os.Getenv("ENABLE_AVIF") == "false" {
			return nil, "", fmt.Errorf("AVIF format is disabled")
		}
		if (width > h.cfg.Constraints.AVIFMaxResolution || height > h.cfg.Constraints.AVIFMaxResolution) || (width*height > h.cfg.Constraints.AVIFMaxPixels) {
			return nil, "", fmt.Errorf("AVIF constraint violation")
		}
		params := vips.NewAvifExportParams()
		if quality > 0 {
			params.Quality = quality
		} else {
			params.Quality = h.cfg.Quality.AVIF
		}
		// Effort (2-6) -> Speed (8-0), where 0 is slowest/best
		params.Speed = (6 - effort) * 2
		buf, _, err = img.ExportAvif(params)
		contentType = "image/avif"

	case "jxl":
		if os.Getenv("ENABLE_JXL") == "false" {
			return nil, "", fmt.Errorf("JXL format is disabled")
		}
		if (width > h.cfg.Constraints.JXLMaxResolution || height > h.cfg.Constraints.JXLMaxResolution) || (width*height > h.cfg.Constraints.JXLMaxPixels) {
			return nil, "", fmt.Errorf("JXL constraint violation")
		}
		params := vips.NewJxlExportParams()
		if quality > 0 {
			params.Quality = quality
		} else {
			params.Quality = h.cfg.Quality.JXL
		}
		// Effort (2-6) -> JXL Effort (1-9), where 9 is slowest/best
		params.Effort = (effort * 2) - 3
		buf, _, err = img.ExportJxl(params)
		contentType = "image/jxl"

	case "webp":
		params := vips.NewWebpExportParams()
		if quality > 0 {
			params.Quality = quality
		} else {
			params.Quality = h.cfg.Quality.WebP
		}
		params.Lossless = lossless
		// Effort (2-6) -> WebP Effort (0-6), where 6 is slowest/best
		params.ReductionEffort = effort
		buf, _, err = img.ExportWebp(params)
		contentType = "image/webp"

	case "png":
		params := vips.NewPngExportParams()
		if quality > 0 {
			params.Quality = quality
		} else {
			params.Quality = h.cfg.Quality.PNG
		}
		buf, _, err = img.ExportPng(params)
		contentType = "image/png"

	default: // jpeg
		params := vips.NewJpegExportParams()
		if quality > 0 {
			params.Quality = quality
		} else {
			params.Quality = h.cfg.Quality.JPEG
		}
		buf, _, err = img.ExportJpeg(params)
		contentType = "image/jpeg"
	}

	if err != nil {
		return nil, "", err
	}

	return buf, contentType, nil
}

// processTask is kept for legacy single-image processing, wrapping the new logic
func (h *ImageHandler) processTask(img *vips.ImageRef, task ResizeTask) ([]byte, string, error) {
	originalWidth := img.Width()
	originalHeight := img.Height()

	targetWidth := task.Width
	targetHeight := task.Height

	if targetWidth == 0 && targetHeight == 0 {
		targetWidth = originalWidth
		targetHeight = originalHeight
	} else if targetWidth > 0 && targetHeight == 0 {
		scale := float64(targetWidth) / float64(originalWidth)
		targetHeight = int(float64(originalHeight) * scale)
	} else if targetHeight > 0 && targetWidth == 0 {
		scale := float64(targetHeight) / float64(originalHeight)
		targetWidth = int(float64(originalWidth) * scale)
	}

	// Resize
	if err := img.Thumbnail(targetWidth, targetHeight, vips.InterestingNone); err != nil {
		return nil, "", fmt.Errorf("failed to resize image")
	}

	finalWidth := img.Width()
	finalHeight := img.Height()
	pixels := finalWidth * finalHeight
	effort := getEffort(pixels)

	return h.exportImage(img, task.Format, task.Quality, task.Lossless, effort, finalWidth, finalHeight)
}
