package helm

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLoadRepo(t *testing.T) {
	var l DirectLoader
	indexFile, err := l.LoadIndexFile("https://aws.github.io/eks-charts")
	require.NoError(t, err)
	require.NotNil(t, indexFile)
	cv, err := indexFile.Get("aws-vpc-cni", "*")
	require.NoError(t, err)
	require.Equal(t, "aws-vpc-cni", cv.Name)
	_, err = indexFile.Get("missing-name", "*")
	require.Error(t, err)
}
