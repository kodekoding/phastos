package image

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Processing interface declares Resize(width, length uint) but
// processing struct implements Resize(width, length int). The interface
// compliance check is skipped — testing the struct methods directly instead.
func TestProcessingStructMethods(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	p := &processing{img: img}
	assert.NotNil(t, p.Image())
	assert.Equal(t, img, p.Image())
}

func TestProcessingStruct(t *testing.T) {
	p := &processing{}
	assert.Nil(t, p.img)
	assert.Nil(t, p.imgFile)
}

func TestProcessingStructWithImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	p := &processing{img: img}
	assert.NotNil(t, p.Image())
	assert.Equal(t, img, p.Image())
}

func TestProcessingImageMethod(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	p := &processing{img: img}
	result := p.Image()
	assert.Equal(t, img, result)
}

func TestProcessingResizeMethod(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	p := &processing{img: img}
	result := p.Resize(100, 100)
	assert.NotNil(t, result)
	bounds := result.Bounds()
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 100, bounds.Dy())
}

func TestProcessingResizeZeroDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	p := &processing{img: img}
	result := p.Resize(0, 0)
	assert.NotNil(t, result)
}

func TestProcessingResizeOneDimension(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	p := &processing{img: img}
	result := p.Resize(50, 0)
	assert.NotNil(t, result)
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/to/file.png")
	assert.Error(t, err)
}

func TestLoadLocalPNGFile(t *testing.T) {
	// Create a temp PNG file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.png")

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 64, B: 32, A: 255})
		}
	}

	f, err := os.Create(filePath)
	require.NoError(t, err)
	err = png.Encode(f, img)
	require.NoError(t, err)
	f.Close()

	loaded, err := Load(filePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	result := loaded.Image()
	assert.NotNil(t, result)
	bounds := result.Bounds()
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 100, bounds.Dy())
}

func TestLoadLocalFileAndResize(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "resize_test.png")

	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	f, err := os.Create(filePath)
	require.NoError(t, err)
	err = png.Encode(f, img)
	require.NoError(t, err)
	f.Close()

	loaded, err := Load(filePath)
	require.NoError(t, err)

	resized := loaded.Resize(50, 50)
	assert.NotNil(t, resized)
	bounds := resized.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func TestLoadInvalidFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.png")

	err := os.WriteFile(filePath, []byte("not a valid image file content"), 0644)
	require.NoError(t, err)

	_, err = Load(filePath)
	assert.Error(t, err)
}

func TestLoadFromHTTPURL(t *testing.T) {
	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 200, B: 50, A: 255})
		}
	}

	// Create a temp PNG file to serve
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "remote.png")
	f, err := os.Create(imgPath)
	require.NoError(t, err)
	err = png.Encode(f, img)
	require.NoError(t, err)
	f.Close()

	// Serve the file via HTTP
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, imgPath)
	}))
	defer server.Close()

	// Load from the HTTP URL with a query string (to test the ? splitting logic)
	loaded, err := Load(server.URL + "/remote.png?param=value")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	result := loaded.Image()
	assert.NotNil(t, result)
	bounds := result.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func TestLoadFromHTTPURLInvalidImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not an image"))
	}))
	defer server.Close()

	_, err := Load(server.URL + "/bad.png?test=1")
	assert.Error(t, err)
}

func TestLoadFromHTTPURLServerDown(t *testing.T) {
	// Use a port that's not listening
	_, err := Load("http://127.0.0.1:1/image.png?test=1")
	assert.Error(t, err)
}
