package storage

import (
	"context"
	"io/fs"
	"mime/multipart"
)

type Buckets interface {
	UploadImage(ctx context.Context, file multipart.File, fileName *string) error
	UploadFile(ctx context.Context, file multipart.File, fileName *string) error

	UploadImageFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error
	UploadFileFromLocalPath(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error

	UploadImagePublic(ctx context.Context, file multipart.File, fileName *string) error
	UploadFilePublic(ctx context.Context, file multipart.File, fileName *string) error
	UploadImageFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error
	UploadFileFromLocalPathPublic(ctx context.Context, filePath string, fileName *string, deleteAfterSuccess ...bool) error

	GetSignedURLFile(ctx context.Context, imgPath string) (signedUrl string, err error)
	GetFileFS(ctx context.Context, filePath string) (fs.File, error)

	SetFileExpiredTime(minutes int) Buckets
	SetBucketName(fileName string) Buckets
	SetContentType(contentType string) Buckets

	RollbackProcess(ctx context.Context, fileName string) error
	DeleteFile(ctx context.Context, fileName string) error

	CopyFileToAnotherBucket(ctx context.Context, destFileName, destBucket, sourceBucket string, optionalParams ...interface{}) error
	Close()
}
