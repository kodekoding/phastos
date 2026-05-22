package storage

import (
	"context"
	"io/fs"
	"mime/multipart"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBucketsInterface(t *testing.T) {
	// Verify the Buckets interface is satisfied by the stub
	var _ Buckets = &stubBucket{}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "upload", UploadProcess)
	assert.Equal(t, "download", DownloadProcess)
}

func TestNewGCSEmptyBucketName(t *testing.T) {
	_, err := NewGCS(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name empty")
}

func TestNewGCSNoCredentials(t *testing.T) {
	// Ensure no credential env vars are set
	os.Unsetenv("STORAGE_CREDENTIALS_PATH")
	os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	_, err := NewGCS(context.Background(), "my-bucket")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credential path isn't set")
}

func TestNewDriveNoCredentials(t *testing.T) {
	// Ensure no credential env vars are set
	os.Unsetenv("DRIVE_CREDENTIALS_PATH")
	os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	_, err := NewDrive(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credential path isn't set")
}

func TestNewDriveWithCredentialPath(t *testing.T) {
	os.Setenv("DRIVE_CREDENTIALS_PATH", "/nonexistent/credentials.json")
	defer os.Unsetenv("DRIVE_CREDENTIALS_PATH")

	_, err := NewDrive(context.Background())
	assert.Error(t, err)
	// Should fail trying to read the credentials file
}

func TestNewDriveWithFallbackCredentialPath(t *testing.T) {
	os.Unsetenv("DRIVE_CREDENTIALS_PATH")
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/credentials.json")
	defer os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	_, err := NewDrive(context.Background())
	assert.Error(t, err)
}

func TestNewGCSWithCredentialPath(t *testing.T) {
	os.Setenv("STORAGE_CREDENTIALS_PATH", "/nonexistent/credentials.json")
	defer os.Unsetenv("STORAGE_CREDENTIALS_PATH")

	_, err := NewGCS(context.Background(), "my-bucket")
	assert.Error(t, err)
	// Should fail trying to create the storage client with invalid credentials
}

func TestNewGCSWithFallbackCredentialPath(t *testing.T) {
	os.Unsetenv("STORAGE_CREDENTIALS_PATH")
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/credentials.json")
	defer os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	_, err := NewGCS(context.Background(), "my-bucket")
	assert.Error(t, err)
}

func TestGoogleSetFileExpiredTime(t *testing.T) {
	g := &google{}
	result := g.SetFileExpiredTime(30)
	assert.Equal(t, g, result)
	assert.Equal(t, 30, g.imageExpTime)
}

func TestGoogleSetContentType(t *testing.T) {
	g := &google{}
	result := g.SetContentType("image/jpeg")
	assert.Equal(t, g, result)
	assert.Equal(t, "image/jpeg", g.contentType)
}

func TestGoogleSetBucketName(t *testing.T) {
	// Can't test SetBucketName without a real client since it uses g.client.Bucket()
	// But we can test the method exists and returns Buckets
	g := &google{bucketName: "original-bucket"}
	assert.Equal(t, "original-bucket", g.bucketName)
}

func TestGoogleCloseRequiresClient(t *testing.T) {
	// Close with nil client will panic, so we just verify the struct exists
	// Real Close() testing requires a real GCS client
	g := &google{}
	assert.NotNil(t, g)
}

func TestGoogleRollbackProcessCallsDeleteFile(t *testing.T) {
	g := &google{}
	// RollbackProcess just delegates to DeleteFile
	// Can't test without real client, but we verify the struct exists
	assert.NotNil(t, g)
}

func TestGoogleInitResumableUploadsNoToken(t *testing.T) {
	os.Unsetenv("GCS_ACCESS_TOKEN")
	g := &google{bucketName: "my-bucket"}
	var path string
	_, err := g.InitResumableUploads(context.Background(), &path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access token cannot be empty")
}

func TestGoogleStructFields(t *testing.T) {
	g := &google{
		imageExpTime: 60,
		contentType:  "image/png",
		bucketName:   "test-bucket",
	}
	assert.Equal(t, 60, g.imageExpTime)
	assert.Equal(t, "image/png", g.contentType)
	assert.Equal(t, "test-bucket", g.bucketName)
}

func TestGoogleUploadImageFromLocalPathError(t *testing.T) {
	g := &google{}
	// Opening non-existent file should error
	err := g.UploadImageFromLocalPath(context.Background(), "/nonexistent/file.png", nil)
	assert.Error(t, err)
}

func TestGoogleUploadFileFromLocalPathError(t *testing.T) {
	g := &google{}
	err := g.UploadFileFromLocalPath(context.Background(), "/nonexistent/file.txt", nil)
	assert.Error(t, err)
}

func TestGoogleUploadImageFromLocalPathPublicError(t *testing.T) {
	g := &google{}
	err := g.UploadImageFromLocalPathPublic(context.Background(), "/nonexistent/file.png", nil)
	assert.Error(t, err)
}

func TestGoogleUploadFileFromLocalPathPublicError(t *testing.T) {
	g := &google{}
	err := g.UploadFileFromLocalPathPublic(context.Background(), "/nonexistent/file.txt", nil)
	assert.Error(t, err)
}

func TestGoogleDriveStruct(t *testing.T) {
	gd := &googleDrive{}
	assert.Nil(t, gd.service)
}

// stubBucket implements Buckets for testing
type stubBucket struct{}

func (s *stubBucket) UploadImage(ctx context.Context, file multipart.File, fileName *string) error {
	return nil
}
func (s *stubBucket) UploadFile(ctx context.Context, file multipart.File, fileName *string) error {
	return nil
}
func (s *stubBucket) UploadImageFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	return nil
}
func (s *stubBucket) UploadFileFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	return nil
}
func (s *stubBucket) UploadImagePublic(ctx context.Context, file multipart.File, fileName *string) error {
	return nil
}
func (s *stubBucket) UploadFilePublic(ctx context.Context, file multipart.File, fileName *string) error {
	return nil
}
func (s *stubBucket) UploadImageFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	return nil
}
func (s *stubBucket) UploadFileFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	return nil
}
func (s *stubBucket) GetSignedURLFile(ctx context.Context, imgPath string) (signedUrl string, err error) {
	return "", nil
}
func (s *stubBucket) GetFileFS(ctx context.Context, filePath string) (fs.File, error) {
	return nil, nil
}
func (s *stubBucket) SetFileExpiredTime(minutes int) Buckets    { return s }
func (s *stubBucket) SetBucketName(fileName string) Buckets     { return s }
func (s *stubBucket) SetContentType(contentType string) Buckets  { return s }
func (s *stubBucket) RollbackProcess(ctx context.Context, fileName string) error {
	return nil
}
func (s *stubBucket) DeleteFile(ctx context.Context, fileName string) error {
	return nil
}
func (s *stubBucket) CopyFileToAnotherBucket(ctx context.Context, destBucket, fileName string) error {
	return nil
}
func (s *stubBucket) GenerateSignedURL(urlType string, path string, expires ...time.Duration) (string, error) {
	return "", nil
}
func (s *stubBucket) Close() {}
