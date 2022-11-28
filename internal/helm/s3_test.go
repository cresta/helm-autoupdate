package helm_test

import (
	"os"
	"testing"

	"github.com/cresta/helm-autoupdate/internal/helm"
	"github.com/stretchr/testify/require"
)

func TestLoadS3(t *testing.T) {
	var l helm.DirectLoader
	s3Repo := os.Getenv("S3_REPO")
	if s3Repo == "" {
		t.Skip("S3_REPO is not set")
	}
	indexFile, err := l.LoadIndexFile(s3Repo)
	require.NoError(t, err)
	require.NotNil(t, indexFile)
	_, err = indexFile.Get("missing-name", "*")
	require.Error(t, err)
}
