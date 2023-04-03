package storage

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
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
}

func (g *google) SetFileExpiredTime(minutes int) Buckets {
	g.imageExpTime = minutes
	return g
}

func NewGCS(ctx context.Context, bucketName string) (Buckets, error) {
	if bucketName == "" {
		return nil, errors.Wrap(errors.New("bucket name empty"), "phastos.go.storage.google.NewGCS.CheckBucketName")
	}
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.storage.google.NewGCS.NewClient")
	}
	return &google{client: gcsClient, bucket: gcsClient.Bucket(bucketName)}, nil
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
	return g.uploadProcess(ctx, file, fileName, "img")
}

func (g *google) UploadFile(ctx context.Context, file multipart.File, fileName *string) error {
	return g.uploadProcess(ctx, file, fileName, "file")
}

func (g *google) UploadImageFromLocalPath(ctx context.Context, filePath string, fileName *string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}
	defer func() {
		_ = os.RemoveAll(filePath)
	}()

	return g.uploadProcess(ctx, file, fileName, "img")
}

func (g *google) UploadFileFromLocalPath(ctx context.Context, filePath string, fileName *string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}
	defer func() {
		_ = os.RemoveAll(filePath)
	}()

	return g.uploadProcess(ctx, file, fileName, "file")
}

func (g *google) uploadProcess(ctx context.Context, file multipart.File, fileName *string, fileType string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	currentEnv := env.ServiceEnv()
	*fileName = fmt.Sprintf("%s/%s/%s", fileType, currentEnv, *fileName)
	writer := g.bucket.Object(*fileName).NewWriter(ctx)

	if g.contentType != "" {
		writer.ContentType = g.contentType
	}
	if _, err := io.Copy(writer, file); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}

	if err := writer.Close(); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Close")
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
