package helper

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"

	"github.com/kodekoding/phastos/env"

	"github.com/pkg/errors"
)

func UploadToTmp(_ context.Context, path *string, file multipart.File) error {
	basePath := ""
	if env.IsLocal() {
		basePath = "files"
	}

	*path = fmt.Sprintf("%s/tmp/%s", basePath, *path)
	CheckFolder(*path)

	tmpFile, err := os.Create(*path)
	if err != nil {
		return errors.Wrap(err, "pkg.helper.upload.UploadToTmp.CreateFile")
	}
	defer tmpFile.Close()

	if _, err = io.Copy(tmpFile, file); err != nil {
		return errors.Wrap(err, "pkg.helper.upload.UploadToTmp.CopyToDestinationFile")
	}

	return nil
}
