package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFolderAndFileName(t *testing.T) {
	t.Run("should split path with file", func(t *testing.T) {
		folder, file := GetFolderAndFileName("path/to/file.txt")
		assert.Equal(t, "path/to/", folder)
		assert.Equal(t, "file.txt", file)
	})

	t.Run("should handle nested path with file", func(t *testing.T) {
		folder, file := GetFolderAndFileName("a/b/c/d/report.pdf")
		assert.Equal(t, "a/b/c/d/", folder)
		assert.Equal(t, "report.pdf", file)
	})

	t.Run("should handle path without file extension (directory)", func(t *testing.T) {
		folder, file := GetFolderAndFileName("path/to/folder")
		assert.Equal(t, "path/to/folder", folder)
		assert.Equal(t, "", file)
	})

	t.Run("should handle single file name", func(t *testing.T) {
		folder, file := GetFolderAndFileName("file.txt")
		assert.Equal(t, "", folder)
		assert.Equal(t, "file.txt", file)
	})

	t.Run("should handle path with dotfile", func(t *testing.T) {
		folder, file := GetFolderAndFileName("path/to/.env")
		assert.Equal(t, "path/to/", folder)
		assert.Equal(t, ".env", file)
	})
}

func TestGetFolderNameWithoutTmp(t *testing.T) {
	t.Run("should extract folder name after tmp", func(t *testing.T) {
		result := GetFolderNameWithoutTmp("files/tmp/uploads/")
		assert.Equal(t, "uploads", result)
	})

	t.Run("should handle nested folders after tmp", func(t *testing.T) {
		result := GetFolderNameWithoutTmp("files/tmp/a/b/")
		assert.Equal(t, "a/b", result)
	})
}
