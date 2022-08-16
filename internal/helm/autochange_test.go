package helm

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestParseLine(t *testing.T) {
	require.Nil(t, ParseLine("asdfdasdsfadsa"))
	require.Equal(t, &LineParse{
		Prefix:         "  version",
		CurrentVersion: "0.3.6",
		Identity:       "datadog",
		Suffix:         "",
	}, ParseLine("  version: 0.3.6 # helm:autoupdate:datadog"))
}

func TestParseLine_String(T *testing.T) {
	require.Equal(T, "  version: 0.3.6 # helm:autoupdate:datadog", ParseLine("  version: 0.3.6 # helm:autoupdate:datadog").String())
}

const cniFile = `apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: aws-vpc-cni
spec:
  chart:
    spec:
      chart: aws-vpc-cni
      sourceRef:
        kind: HelmRepository
        name: aws-vpc-cni
      version: 0.3.6 # helm:autoupdate:aws-vpc-cni
  interval: 1m0s
  timeout: 10m0s # Lots of pods in the daemonset
  values:
    a: b
`

func cniFileMatchesExpected(t *testing.T, pf *ParsedFile) {
	require.Equal(t, cniFile, pf.OriginalContent)
	require.Equal(t, "  chart:", pf.Lines[5])
	require.Equal(t, []Update{
		{
			LineNumber: 11,
			Parse: &LineParse{
				Prefix:         "      version",
				CurrentVersion: "0.3.6",
				Identity:       "aws-vpc-cni",
				Suffix:         "",
			},
		},
	}, pf.RequestedUpdates)
}

func TestParseContent(t *testing.T) {
	pf := ParseContent(cniFile)
	cniFileMatchesExpected(t, &pf)
}

func TestParseFile(t *testing.T) {
	f, err := ioutil.TempFile("", "TestParseFile")
	require.NoError(t, err)
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			panic(err)
		}
	}(f.Name())
	require.NoError(t, ioutil.WriteFile(f.Name(), []byte(cniFile), 0600))
	pf, err := ParseFile(f.Name())
	require.NoError(t, err)
	cniFileMatchesExpected(t, pf)
}

func TestApplyUpdate(t *testing.T) {
	pf := ParseContent(cniFile)
	pf.ApplyUpdate(&Update{
		LineNumber: 11,
		Parse: &LineParse{
			Prefix:         "      version",
			CurrentVersion: "0.0.0",
			Identity:       "aws-vpc-cni",
			Suffix:         "",
		},
	})
	require.Contains(t, string(pf.Bytes()), "version: 0.0.0")
}

const testConfig = `charts:
- chart:
    name: aws-vpc-cni
    repository: https://aws.github.io/eks-charts
    version: 1.0.5
  identity: aws-vpc-cni
- chart:
    name: datadog
    repository: https://helm.datadoghq.com
    version: 1.0.0
  identity: datadog
filename_regex:
- .*\.yaml
`

func TestLoadFile(t *testing.T) {
	f, err := ioutil.TempFile("", "TestLoadFile")
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			panic(err)
		}
	}(f.Name())
	require.NoError(t, ioutil.WriteFile(f.Name(), []byte(testConfig), 0600))
	ac, err := LoadFile(f.Name())
	require.NoError(t, err)
	b, err := yaml.Marshal(ac)
	require.NoError(t, err)
	require.Equal(t, testConfig, string(b))
}

func generateExample(t *testing.T) (string, func()) {
	dirName, err := ioutil.TempDir("", "generateExample")
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(filepath.Join(dirName, ".helm-autoupdate.yaml"), []byte(testConfig), 0600))
	require.NoError(t, ioutil.WriteFile(filepath.Join(dirName, "aws-vpc-cni.yaml"), []byte(cniFile), 0600))
	require.NoError(t, ioutil.WriteFile(filepath.Join(dirName, "test-example.yaml"), []byte(`name: jack`), 0600))
	return dirName, func() {
		err := os.RemoveAll(dirName)
		if err != nil {
			panic(err)
		}
	}
}

func TestFindRequestedChanges(t *testing.T) {
	dirName, cleanup := generateExample(t)
	defer cleanup()
	x := DirectorySearchForChanges{
		Dir: dirName,
	}
	changeFiles, err := x.FindRequestedChanges(nil)
	require.NoError(t, err)
	require.Len(t, changeFiles, 1)
	require.Equal(t, filepath.Join(dirName, "aws-vpc-cni.yaml"), changeFiles[0].OriginalFilename)
}

func TestWriteChangesToFilesystem(t *testing.T) {
	dirName, cleanup := generateExample(t)
	defer cleanup()
	x := DirectorySearchForChanges{
		Dir: dirName,
	}
	changeFiles, err := x.FindRequestedChanges(nil)
	require.NoError(t, err)
	ru := changeFiles[0].RequestedUpdates[0]
	ru.Parse.CurrentVersion = "99.99.99"
	changeFiles[0].ApplyUpdate(&ru)
	require.NoError(t, WriteChangesToFilesystem(changeFiles))
	b, err := ioutil.ReadFile(filepath.Join(dirName, "aws-vpc-cni.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(b), "      version: 99.99.99 # helm:autoupdate:aws-vpc-cni")
}

func TestFindUpdateChartForUpdate(t *testing.T) {
	ac, err := Load([]byte(testConfig))
	require.NoError(t, err)
	require.Nil(t, ac.findUpdateChartForUpdate(&Update{
		Parse: &LineParse{},
	}))
	x := ac.findUpdateChartForUpdate(&Update{
		Parse: &LineParse{
			Identity: "blarg",
		},
	})
	require.Nil(t, x)
	x = ac.findUpdateChartForUpdate(&Update{
		Parse: &LineParse{
			Identity: "aws-vpc-cni",
		},
	})
	require.NotNil(t, x)
}

func TestApplyUpdatesToFiles(t *testing.T) {
	dirName, cleanup := generateExample(t)
	defer cleanup()
	ac, err := LoadFile(filepath.Join(dirName, ".helm-autoupdate.yaml"))
	require.NoError(t, err)

	var l DirectLoader
	x := DirectorySearchForChanges{
		Dir: dirName,
	}
	pf, err := x.FindRequestedChanges(ac.ParsedRegex)
	require.NoError(t, err)
	require.Len(t, pf, 1)

	updatedFiles, err := ApplyUpdatesToFiles(&l, ac, pf)
	require.NoError(t, err)
	require.Len(t, updatedFiles, 1)
	require.NoError(t, WriteChangesToFilesystem(updatedFiles))
	b, err := ioutil.ReadFile(filepath.Join(dirName, "aws-vpc-cni.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(b), "      version: 1.0.5 # helm:autoupdate:aws-vpc-cni")
}
