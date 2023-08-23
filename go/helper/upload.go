package helper

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"

	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/kodekoding/phastos/v2/go/storage"
)

type CloudStorageTmpUpload struct {
	Storage       storage.Buckets
	TmpBucketName string
}

func UploadToTmp(ctx context.Context, path *string, file multipart.File, cloudStorage ...*CloudStorageTmpUpload) error {
	basePath := ""
	if env.IsLocal() {
		basePath = "files"
	}
	basePath += "/"

	isTmpToCloudStorage := cloudStorage != nil && len(cloudStorage) > 0
	if isTmpToCloudStorage {
		basePath = ""
	}

	if isTmpToCloudStorage {
		*path = fmt.Sprintf("tmp/%s", *path)
		cloudStorageOption := cloudStorage[0]
		store := cloudStorageOption.Storage
		if err := store.SetBucketName(cloudStorageOption.TmpBucketName).UploadFile(ctx, file, path); err != nil {
			return errors.Wrap(err, "phastos.go.helper.upload.UploadToTmp.UploadFileGCS")
		}
	} else {
		*path = fmt.Sprintf("%stmp/%s", basePath, *path)
		CheckFolder(*path)

		tmpFile, err := os.Create(*path)
		if err != nil {
			return errors.Wrap(err, "pkg.helper.upload.UploadToTmp.CreateFile")
		}
		defer tmpFile.Close()

		if _, err = io.Copy(tmpFile, file); err != nil {
			return errors.Wrap(err, "phastos.go.helper.upload.UploadToTmp.CopyToDestinationFile")
		}
	}

	return nil
}
