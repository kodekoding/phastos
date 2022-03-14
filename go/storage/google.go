package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/go/env"
)

type google struct {
	client *storage.BucketHandle
}

func NewGCS(ctx context.Context, bucketName string) (Buckets, error) {
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.storage.google.NewGCS.NewClient")
	}
	return &google{client: gcsClient.Bucket(bucketName)}, nil
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
		_ = file.Close()
		_ = os.Remove(filePath)
	}()

	return g.uploadProcess(ctx, file, fileName, "img")
}

func (g *google) uploadProcess(ctx context.Context, file multipart.File, fileName *string, fileType string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	currentEnv := env.ServiceEnv()
	*fileName = fmt.Sprintf("%s/%s/%s", fileType, currentEnv, *fileName)
	writer := g.client.Object(*fileName).NewWriter(ctx)
	if _, err := io.Copy(writer, file); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Copy")
	}

	if err := writer.Close(); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.Upload.Close")
	}

	return nil
}

func (g *google) GetFile(ctx context.Context, imgPath string) (signedUrl string, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(15 * time.Minute),
	}

	signedUrl, err = g.client.SignedURL(imgPath, opts)
	if err != nil {
		err = errors.Wrap(err, "pkg.uti.storage.google.GetFile.GetSignedURL")
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

	if err := g.client.Object(fileName).Delete(ctx); err != nil {
		return errors.Wrap(err, "phastos.go.storage.google.DeleteObject")
	}
	return nil
}
