# helm-autoupdate

CLI/action to update helm versions in git repositories

# Motivation

You start with a helm release object
```yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: aws-vpc-cni
spec:
  chart:
    spec:
      chart: aws-vpc-cni
      sourceRef:
        kind: HelmRepository
        name: aws
      version: 0.0.1
  interval: 1m0s
  timeout: 10
  values:
    replicaCount: 1
```

This is fine, but how do you know when to update the helm release to a newer version?  One option is to use `*` like this
```yaml
      sourceRef:
        kind: HelmRepository
        name: aws
      version: "*"
```

But in this case, you don't have any git tracking of what version was released.  What you really want is some automation
that will bump the `version` field when a new helm chart is released.  This is what `helm-autoupdate` is for.

First, add a file named `.helm-autoupdate.yaml` in the root of your repository.  Add a `chart` item for each chart you want to update.

```yaml
charts:
- chart:
    name: aws-vpc-cni
    repository: https://aws.github.io/eks-charts
    version: "*"
  identity: aws-vpc-cni
filename_regex:
- .*\.yaml
```

Next, change the `version` line to include the YAML comment `#helm:autoupdate:<IDENTITY>` where `<IDENTITY>` is the value
of the `charts[].identity` field.  For example, the original file now becomes

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: aws-vpc-cni
spec:
  chart:
    spec:
      chart: aws-vpc-cni
      sourceRef:
        kind: HelmRepository
        name: aws
      version: 0.0.1 # helm:autoupdate:aws-vpc-cni
  interval: 1m0s
  timeout: 10
  values:
    replicaCount: 1
```

Next, triger a run of `helm-autoupdate`.  One way is to compile and run the binary with `go run`.  For example

```bash
cd /tmp
git clone git@github.com:cresta/helm-autoupdate.git
go build ./cmd/helm-autoupdate
cd -
/tmp/helm-autoupdate
```

If you're using GitHub actions, a more reasonable way is to trigger the update as a workflow.  An example workflow is
below.  This will trigger on a manual execution of the workflow, as well as daily at midnight.

```yaml
name: Force a helm update
on:
  workflow_dispatch:
  schedule:
    - cron: "0 0 * * *"
jobs:
  plantrigger:
    runs-on: ubuntu-latest
    name: Force helm update
    steps:
      - name: Checkout
        uses: actions/checkout@v2
      - name: update helm
        uses: cresta/helm-autoupdate@v1.6
      - name: Create PR with changes
        uses: peter-evans/create-pull-request@v3
        id: cpr
        with:
          branch: helm-updates
          delete-branch: true
          title: "Force helm updates"
          labels: forced-workflow
          committer: Forced updates <noreply@noreply.com>
          body: "Updated helm versions"
          commit-message: "Updates helm versions"

```

You can combine this with GitHub's auto-merge feature and status checks to complete the auto merge.

# Usage

For a simple example, see the workflow file in [helm-autoupdate-testing](https://github.com/cresta/helm-autoupdate-testing/blob/main/.github/workflows/update-helm-versions.yaml)
