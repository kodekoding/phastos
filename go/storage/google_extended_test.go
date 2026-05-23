package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	gcs "cloud.google.com/go/storage"
)

type mockMultipartFile struct {
	*bytes.Reader
}

func (m *mockMultipartFile) Close() error {
	return nil
}

type rewriteTransport struct {
	targetURL string
	transport http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "storage.googleapis.com" {
		target, _ := url.Parse(t.targetURL)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}
	return t.transport.RoundTrip(req)
}

func TestGoogleStorageExtended(t *testing.T) {
	// Start local mock GCS server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("MOCK_GCS: %s %s?%s\n", r.Method, r.URL.Path, r.URL.RawQuery)

		// 1. Resumable upload initialization
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/upload/storage/v1/b/test-bucket/o") {
			if r.URL.Query().Get("uploadType") == "resumable" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Location", fmt.Sprintf("http://%s/upload-target?upload_id=mock-upload-id", r.Host))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(fmt.Sprintf(`"http://%s/upload-target?upload_id=mock-upload-id"`, r.Host)))
				return
			}
		}

		// 2. Upload target (PUT to upload-target or directly simple upload)
		if (r.Method == "PUT" && r.URL.Path == "/upload-target") || (r.Method == "POST" && strings.Contains(r.URL.Path, "/upload/storage/v1/b/test-bucket/o")) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"test-file", "bucket":"test-bucket"}`))
			return
		}

		// 3. ACL update (make public)
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/acl/allUsers") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"role":"READER", "entity":"allUsers"}`))
			return
		}

		// 4. Copy file
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/rewriteTo/b/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"done": true, "resource": {"name":"copied-file", "size":"100"}}`))
			return
		}

		// 5. Delete file
		if r.Method == "DELETE" && (strings.Contains(r.URL.Path, "/b/test-bucket/o/") || strings.Contains(r.URL.Path, "/storage/v1/b/test-bucket/o/")) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 6. Object metadata (JSON API GET object) and direct download
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/b/test-bucket/o/") {
			if r.URL.Query().Get("alt") == "media" {
				// Direct download with alt=media: return raw content
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("mock file content"))
				return
			}
			// JSON object metadata
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"file.txt","bucket":"test-bucket","size":"16","contentType":"text/plain"}`))
			return
		}

		// 7. Object listing (JSON API list objects)
		if r.Method == "GET" && r.URL.Path == "/b/test-bucket/o" && r.URL.Query().Get("alt") == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"kind":"storage#objects","prefixes":null,"items":null}`))
			return
		}

		// 8. Direct download via storage.googleapis.com/<bucket>/<object>
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/test-bucket/") {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mock file content"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := context.Background()

	// Instantiate client pointing to local mock server
	gcsClient, err := gcs.NewClient(ctx, option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	restyClient := resty.New()
	restyClient.GetClient().Transport = &rewriteTransport{
		targetURL: ts.URL,
		transport: http.DefaultTransport,
	}

	g := &google{
		client:     gcsClient,
		bucket:     gcsClient.Bucket("test-bucket"),
		resty:      restyClient,
		bucketName: "test-bucket",
	}
	defer g.Close()

	// Test SetFileExpiredTime, SetContentType, SetBucketName
	g.SetFileExpiredTime(10)
	g.SetContentType("text/plain")
	g.SetBucketName("test-bucket")

	// Test UploadImage
	t.Run("UploadImage", func(t *testing.T) {
		file := &mockMultipartFile{Reader: bytes.NewReader([]byte("image-data"))}
		filename := "image.png"
		err := g.UploadImage(ctx, file, &filename)
		assert.NoError(t, err)
		assert.Contains(t, filename, "private/img")
	})

	// Test UploadFile
	t.Run("UploadFile", func(t *testing.T) {
		file := &mockMultipartFile{Reader: bytes.NewReader([]byte("file-data"))}
		filename := "doc.pdf"
		err := g.UploadFile(ctx, file, &filename)
		assert.NoError(t, err)
		assert.Contains(t, filename, "private/file")
	})

	// Test UploadImagePublic
	t.Run("UploadImagePublic", func(t *testing.T) {
		file := &mockMultipartFile{Reader: bytes.NewReader([]byte("image-data-public"))}
		filename := "img-public.png"
		err := g.UploadImagePublic(ctx, file, &filename)
		assert.NoError(t, err)
		assert.Contains(t, filename, "public/img")
	})

	// Test UploadFilePublic
	t.Run("UploadFilePublic", func(t *testing.T) {
		file := &mockMultipartFile{Reader: bytes.NewReader([]byte("file-data-public"))}
		filename := "file-public.pdf"
		err := g.UploadFilePublic(ctx, file, &filename)
		assert.NoError(t, err)
		assert.Contains(t, filename, "public/file")
	})

	// Test UploadImageFromLocalPath
	t.Run("UploadImageFromLocalPath", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-img-*.png")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, _ = tmpFile.Write([]byte("local-image-data"))
		_ = tmpFile.Close()

		filename := "local-image.png"
		err = g.UploadImageFromLocalPath(ctx, tmpFile.Name(), &filename, false)
		assert.NoError(t, err)
	})

	// Test UploadFileFromLocalPath
	t.Run("UploadFileFromLocalPath", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-file-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, _ = tmpFile.Write([]byte("local-file-data"))
		_ = tmpFile.Close()

		filename := "local-file.txt"
		err = g.UploadFileFromLocalPath(ctx, tmpFile.Name(), &filename, false)
		assert.NoError(t, err)
	})

	// Test UploadImageFromLocalPathPublic
	t.Run("UploadImageFromLocalPathPublic", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-img-pub-*.png")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, _ = tmpFile.Write([]byte("local-image-data-pub"))
		_ = tmpFile.Close()

		filename := "local-image-pub.png"
		err = g.UploadImageFromLocalPathPublic(ctx, tmpFile.Name(), &filename, false)
		assert.NoError(t, err)
	})

	// Test UploadFileFromLocalPathPublic
	t.Run("UploadFileFromLocalPathPublic", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-file-pub-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, _ = tmpFile.Write([]byte("local-file-data-pub"))
		_ = tmpFile.Close()

		filename := "local-file-pub.txt"
		err = g.UploadFileFromLocalPathPublic(ctx, tmpFile.Name(), &filename, false)
		assert.NoError(t, err)
	})

	// Test ReadFile
	t.Run("ReadFile", func(t *testing.T) {
		content, err := g.ReadFile(ctx, "some-path/file.txt")
		assert.NoError(t, err)
		assert.Equal(t, "mock file content", string(content))
	})

	// Test RollbackProcess and DeleteFile
	t.Run("DeleteAndRollback", func(t *testing.T) {
		err := g.DeleteFile(ctx, "delete-this.txt")
		assert.NoError(t, err)

		err = g.RollbackProcess(ctx, "rollback-this.txt")
		assert.NoError(t, err)
	})

	// Test CopyFileToAnotherBucket
	t.Run("CopyFileToAnotherBucket", func(t *testing.T) {
		err := g.CopyFileToAnotherBucket(ctx, "dest-bucket", "src-file.txt")
		assert.NoError(t, err)
	})

	// Test InitResumableUploads
	t.Run("InitResumableUploads", func(t *testing.T) {
		os.Setenv("GCS_ACCESS_TOKEN", "mock-token")
		defer os.Unsetenv("GCS_ACCESS_TOKEN")

		path := "target-path.zip"
		sessionURI, err := g.InitResumableUploads(ctx, &path)
		assert.NoError(t, err)
		assert.Contains(t, sessionURI, "/upload-target?upload_id=mock-upload-id")
	})

	// Test GenerateSignedURL & GetSignedURLFile
	t.Run("SignedURL", func(t *testing.T) {
		// Mock SignedURL doesn't call endpoint, but it requires Google credentials (private key) configured on client to sign locally
		// Since we used option.WithoutAuthentication(), SignedURL will fail, which is expected and good to cover error path
		_, err1 := g.GenerateSignedURL(UploadProcess, "some-path.zip")
		assert.Error(t, err1)

		_, err2 := g.GetSignedURLFile(ctx, "some-path.png")
		assert.Error(t, err2)
	})

	// Test GetFileFS
	t.Run("GetFileFS", func(t *testing.T) {
		file, err := g.GetFileFS(ctx, "some-path/file.txt")
		assert.NoError(t, err)
		if err == nil {
			defer file.Close()
			content, err := io.ReadAll(file)
			assert.NoError(t, err)
			assert.Equal(t, "mock file content", string(content))
		}
	})
}
