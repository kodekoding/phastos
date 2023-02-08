package storage

import (
	"context"
	"mime/multipart"
)

type Buckets interface {
	UploadImage(ctx context.Context, file multipart.File, fileName *string) error
	UploadFile(ctx context.Context, file multipart.File, fileName *string) error

	UploadImageFromLocalPath(ctx context.Context, filePath string, fileName *string) error
	UploadFileFromLocalPath(ctx context.Context, filePath string, fileName *string) error

	GetFile(ctx context.Context, imgPath string) (base64Result string, err error)
	SetFileExpiredTime(minutes int) Buckets

	RollbackProcess(ctx context.Context, fileName string) error
	DeleteFile(ctx context.Context, fileName string) error

	CopyFileToAnotherBucket(ctx context.Context, destBucket, fileName string) error
}
