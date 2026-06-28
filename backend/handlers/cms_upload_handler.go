package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lapakgaming/i18n-center/services"
)

type CmsUploadHandler struct {
	gcs *services.GCSService
}

// NewCmsUploadHandler creates the upload handler. Returns nil + error if GCS is not configured.
func NewCmsUploadHandler() (*CmsUploadHandler, error) {
	gcs, err := services.NewGCSService()
	if err != nil {
		return nil, err
	}
	return &CmsUploadHandler{gcs: gcs}, nil
}

var allowedImageTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// UploadImage handles multipart image upload for CMS rich text fields.
// POST /cms/upload-image
// Form field: "file" (required)
// Returns: { "url": "https://img.lapakgaming.com/s/cms/..." }
// @Summary      Upload CMS image
// @Description  Uploads an image to GCS and returns the PixelShift CDN URL. Max 10 MB. Allowed types: JPEG, PNG, GIF, WebP
// @Tags         cms
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file  formData  file    true  "Image file (JPEG, PNG, GIF, WebP — max 10 MB)"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /cms/upload-image [post]
func (h *CmsUploadHandler) UploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}

	if fileHeader.Size > 10<<20 { // 10 MB limit
		c.JSON(http.StatusBadRequest, gin.H{"error": "file size exceeds 10 MB limit"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// Detect content type from first 512 bytes
	contentType := http.DetectContentType(data)
	// Normalise: http.DetectContentType may return "image/jpeg" or similar
	contentType = strings.Split(contentType, ";")[0]
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		// Fall back to file extension
		extFromName := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileHeader.Filename), "."))
		for _, v := range allowedImageTypes {
			if v == extFromName {
				ext = extFromName
				ok = true
				break
			}
		}
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported image type: %s", contentType)})
			return
		}
	}

	filename := uuid.New().String() + "." + ext
	publicURL, err := h.gcs.Upload(c.Request.Context(), filename, contentType, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload image: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": publicURL})
}
