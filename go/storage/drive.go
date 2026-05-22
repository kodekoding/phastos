package storage

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type (
	googleDrive struct {
		service *drive.Service
	}
)

func NewDrive(ctx context.Context) (*googleDrive, error) {

	driveCredentialPath := os.Getenv("DRIVE_CREDENTIALS_PATH")
	if driveCredentialPath == "" {
		// get default credential path
		driveCredentialPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}

	if driveCredentialPath == "" {
		// if credential path still empty, then throw error
		return nil, errors.Wrap(errors.New("credential path isn't set !"), "phastos.go.storage.drive.NewDrive.CheckCredentialPath")
	}
	driveService, err := drive.NewService(ctx, option.WithCredentialsFile(driveCredentialPath))
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.storage.drive.NewDrive.NewService")
	}

	//restyClient := resty.New()
	return &googleDrive{service: driveService}, nil
}
