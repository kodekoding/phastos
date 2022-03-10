package helper

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/go/env"
)

func GetFilePath() string {
	if env.IsLocal() {
		return "files"
	}
	return ""
}

func CheckFolder(folderPath string) {
	folderPath, _ = GetFolderAndFileName(folderPath)

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		_ = os.MkdirAll(folderPath, 0755)
	}
}

func GetFolderAndFileName(path string) (folderPath string, fileName string) {
	splitPath := strings.Split(path, "/")
	if strings.Contains(path, ".") {
		lastIndex := len(splitPath) - 1
		for x := 0; x < lastIndex; x++ {
			folderPath = fmt.Sprintf("%s%s/", folderPath, splitPath[x])
		}
		fileName = splitPath[lastIndex]
	} else {
		folderPath = path
	}

	return
}
func GetFolderNameWithoutTmp(path string) (folderName string) {
	folderName = path[strings.Index(path, "tmp")+4 : len(path)-1]
	return
}

func GetTmpFolderPath() (string, error) {
	tmpFolderPath := fmt.Sprintf("%s/tmp", GetFilePath())

	if _, err := os.Stat(tmpFolderPath); os.IsNotExist(err) {
		errDir := os.Mkdir(tmpFolderPath, 0777)
		if errDir != nil {
			return "", errors.Wrap(errDir, "phastos.helper.path.CreateFolder")
		}
	}

	return tmpFolderPath, nil
}
