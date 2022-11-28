package helm_test

import (
	"testing"

	"github.com/cresta/helm-autoupdate/internal/helm"
	"github.com/stretchr/testify/require"
)

func TestLoadRepo(t *testing.T) {
	var l helm.DirectLoader
	indexFile, err := l.LoadIndexFile("https://aws.github.io/eks-charts")
	require.NoError(t, err)
	require.NotNil(t, indexFile)
	cv, err := indexFile.Get("aws-vpc-cni", "*")
	require.NoError(t, err)
	require.Equal(t, "aws-vpc-cni", cv.Name)
	_, err = indexFile.Get("missing-name", "*")
	require.Error(t, err)
}
