package helper

import (
	"os"
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

// ---- GetFilePath tests ----

func TestGetFilePath(t *testing.T) {
	t.Run("should return 'files' when IsLocal", func(t *testing.T) {
		// The actual behavior depends on env.IsLocal(), which in tests
		// depends on the ENV environment variable
		envVal := os.Getenv("ENV")
		if envVal == "local" {
			result := GetFilePath()
			assert.Equal(t, "files", result)
		}
	})

	t.Run("should return empty string when not local", func(t *testing.T) {
		envVal := os.Getenv("ENV")
		if envVal != "local" {
			result := GetFilePath()
			assert.Equal(t, "", result)
		}
	})
}

// ---- CheckFolder tests ----

func TestCheckFolder(t *testing.T) {
	t.Run("should create folder when it does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := tmpDir + "/test_nested/deep/folder"

		CheckFolder(testPath)

		// Verify folder was created
		info, err := os.Stat(testPath)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("should not error when folder already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFolder := tmpDir + "/existing"
		err := os.MkdirAll(existingFolder, 0755)
		assert.NoError(t, err)

		// Should not panic or error
		assert.NotPanics(t, func() {
			CheckFolder(existingFolder)
		})
	})
}

// ---- GetTmpFolderPath tests ----

func TestGetTmpFolderPath(t *testing.T) {
	t.Run("should return tmp folder path", func(t *testing.T) {
		path, err := GetTmpFolderPath()
		assert.NoError(t, err)
		assert.Contains(t, path, "tmp")
	})

	t.Run("should return same path on subsequent calls", func(t *testing.T) {
		path1, err := GetTmpFolderPath()
		assert.NoError(t, err)

		path2, err := GetTmpFolderPath()
		assert.NoError(t, err)

		assert.Equal(t, path1, path2)
	})
}
