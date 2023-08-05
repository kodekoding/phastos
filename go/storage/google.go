package storage

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"google.golang.org/api/option"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/mauri870/gcsfs"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/go/env"
)

type google struct {
	client       *storage.Client
	bucket       *storage.BucketHandle
	imageExpTime int
	contentType  string
	resty        *resty.Client
	bucketName   string
}

func (g *google) SetFileExpiredTime(minutes int) Buckets {
	g.imageExpTime = minutes
	return g
}

func NewGCS(ctx context.Context, bucketName string) (Buckets, error) {
	if bucketName == "" {
		return nil, errors.Wrap(errors.New("bucket name empty"), "phastos.go.storage.google.NewGCS.CheckBucketName")
	}

	storageCredentialsPath := os.Getenv("STORAGE_CREDENTIALS_PATH")
	if storageCredentialsPath == "" {
		// get default credential path
		storageCredentialsPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}

	if storageCredentialsPath == "" {
		// if credential path still empty, then throw error
		return nil, errors.Wrap(errors.New("credential path isn't set !"), "phastos.go.storage.google.NewGCS.CheckCredentialPath")
	}
	gcsClient, err := storage.NewClient(ctx, option.WithCredentialsFile(storageCredentialsPath))
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.storage.google.NewGCS.NewClient")
	}

	restyClient := resty.New()
	return &google{
		client:     gcsClient,
		bucket:     gcsClient.Bucket(bucketName),
		resty:      restyClient,
		bucketName: bucketName,
	}, nil
}

func (g *google) Close() {
	_ = g.client.Close()
}

func (g *google) SetContentType(contentType string) Buckets {
	g.contentType = contentType
	return g
}

// SetBucketName - to update/change the initial bucket name
func (g *google) SetBucketName(fileName string) Buckets {
	g.bucket = g.client.Bucket(fileName)
	return g
}

func (g *google) UploadImage(ctx context.Context, file multipart.File, fileName *string) error {
	return g.uploadProcess(ctx, file, fileName, "private/img")
}

func (g *google) UploadFile(ctx context.Context, file multipart.File, fileName *string) error {
	return g.uploadProcess(ctx, file, fileName, "private/file")
}

func (g *google) UploadImageFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}
	defer func() {
		if deleteAfterSuccess != nil && len(deleteAfterSuccess) > 0 && deleteAfterSuccess[0] {
			_ = os.RemoveAll(filePath)
		}
	}()

	return g.uploadProcess(ctx, file, fileName, "private/img")
}

func (g *google) UploadFileFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}
	defer func() {
		if deleteAfterSuccess != nil && len(deleteAfterSuccess) > 0 && deleteAfterSuccess[0] {
			_ = os.RemoveAll(filePath)
		}
	}()

	return g.uploadProcess(ctx, file, fileName, "private/file")
}

func (g *google) UploadImagePublic(ctx context.Context, file multipart.File, fileName *string) error {
	return g.uploadProcess(ctx, file, fileName, "public/img")
}

func (g *google) UploadFilePublic(ctx context.Context, file multipart.File, fileName *string) error {
	return g.uploadProcess(ctx, file, fileName, "public/file")
}

func (g *google) UploadImageFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.UploadImageFromLocalPathPublic.PublicCopy")
	}
	defer func() {
		if deleteAfterSuccess != nil && len(deleteAfterSuccess) > 0 && deleteAfterSuccess[0] {
			_ = os.RemoveAll(filePath)
		}
	}()

	return g.uploadProcess(ctx, file, fileName, "public/img")
}

func (g *google) UploadFileFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.UploadImageFromLocalPathPublic.Copy")
	}
	defer func() {
		if deleteAfterSuccess != nil && len(deleteAfterSuccess) > 0 && deleteAfterSuccess[0] {
			_ = os.RemoveAll(filePath)
		}
	}()

	return g.uploadProcess(ctx, file, fileName, "public/file")
}

func (g *google) uploadProcess(ctx context.Context, file multipart.File, fileName *string, fileType string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	currentEnv := env.ServiceEnv()
	splitType := strings.Split(fileType, "/")
	isPublic := false
	if splitType[0] == "public" {
		isPublic = true
	}

	*fileName = fmt.Sprintf("%s/%s/%s", fileType, currentEnv, *fileName)
	obj := g.bucket.Object(*fileName).NewWriter(ctx)

	if g.contentType != "" {
		obj.ContentType = g.contentType
	}
	if _, err := io.Copy(obj, file); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}

	if err := obj.Close(); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Close")
	}
	if isPublic {
		_ = g.makeObjectPublic(ctx, *fileName)
	}

	return nil
}

func (g *google) makeObjectPublic(ctx context.Context, filename string) error {
	if err := g.bucket.Object(filename).ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		return err
	}

	return nil
}

func (g *google) ReadFile(ctx context.Context, filePath string) ([]byte, error) {
	reader, err := g.bucket.Object(filePath).NewReader(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.pkg.uti.storage.google.ReadFile.NewReader")
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.pkg.uti.storage.google.ReadFile.ReadContent")
	}
	return content, nil
}

func (g *google) GetFileFS(ctx context.Context, filePath string) (fs.File, error) {
	return gcsfs.NewWithBucketHandle(g.bucket).WithContext(ctx).Open(filePath)
}

func (g *google) GetSignedURLFile(ctx context.Context, imgPath string) (signedUrl string, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	imgExpiredTime := 60
	if g.imageExpTime != 0 {
		imgExpiredTime = g.imageExpTime
	}

	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(time.Duration(imgExpiredTime) * time.Minute),
	}

	signedUrl, err = g.bucket.SignedURL(imgPath, opts)
	if err != nil {
		err = errors.Wrap(err, "phastos.pkg.uti.storage.google.GetFile.GetSignedURL")
		return
	}

	return
}

func (g *google) RollbackProcess(ctx context.Context, fileName string) error {
	return g.DeleteFile(ctx, fileName)
}

func (g *google) DeleteFile(ctx context.Context, fileName string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	if err := g.bucket.Object(fileName).Delete(ctx); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.DeleteObject")
	}
	return nil
}

func (g *google) CopyFileToAnotherBucket(ctx context.Context, destBucket, fileName string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	src := g.bucket.Object(fileName)
	dst := g.client.Bucket(destBucket).Object(fileName).If(storage.Conditions{DoesNotExist: true})

	if _, err := dst.CopierFrom(src).Run(ctx); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.CopyFileToAnotherBucket")
	}
	return nil
}

func (g *google) InitResumableUploads(ctx context.Context, gcsPath *string) (string, error) {
	gcsAccessToken := os.Getenv("GCS_ACCESS_TOKEN")
	if gcsAccessToken == "" {
		return "", errors.Wrap(errors.New("access token cannot be empty"), "phastos.go.storage.google.InitResumableUploads.GetAccessTokenFromEnv")
	}

	*gcsPath = fmt.Sprintf("resumable/%s/%s", env.ServiceEnv(), *gcsPath)

	gcsURL := fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=resumable&name=%s", g.bucketName, *gcsPath)
	var sessionURI string
	if _, err := g.resty.R().
		SetContext(ctx).
		SetResult(&sessionURI).
		SetHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", gcsAccessToken),
			"Content-Type":  "application/json",
		}).
		Post(gcsURL); err != nil {
		return "", errors.Wrap(err, "phastos.go.storage.google.InitResumableUploads.Post")
	}
	return sessionURI, nil
}
