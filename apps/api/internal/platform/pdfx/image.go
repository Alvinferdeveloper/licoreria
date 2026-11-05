package pdfx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
)

const maxImageBytes = 2 << 20

// Image is a picture ready to draw in a document.
type Image struct {
	Data []byte
	Type extension.Type
}

// ImageCache keeps the last downloaded image: uploads get a new URL, so it never goes stale.
type ImageCache struct {
	mu    sync.Mutex
	url   string
	image *Image
}

// Get returns the PNG or JPEG at url, downloading it only when the URL changes.
func (c *ImageCache) Get(ctx context.Context, url string) (*Image, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.url == url && c.image != nil {
		return c.image, nil
	}
	image, err := download(ctx, url)
	if err != nil {
		return nil, err
	}
	c.url, c.image = url, image
	return image, nil
}

func download(ctx context.Context, url string) (*Image, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download image: status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("download image: larger than %d bytes", maxImageBytes)
	}
	switch kind := http.DetectContentType(data); kind {
	case "image/png":
		return &Image{Data: data, Type: extension.Png}, nil
	case "image/jpeg":
		return &Image{Data: data, Type: extension.Jpg}, nil
	default:
		return nil, fmt.Errorf("download image: unsupported type %s", kind)
	}
}
