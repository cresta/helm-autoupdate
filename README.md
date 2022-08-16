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

# Usage

First, add a file named `.helm-autoupdate.yaml` in the root of your repository.  Add a `chart` item for each chart you want to update.
The field "filename_regex" is an optional list of whitelisted filenames.  If you don't specify it, all files will be considered.

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

Next, change the `version` line to include the YAML comment `# helm:autoupdate:<IDENTITY>` where `<IDENTITY>` is the value
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
  timeout: 10m
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

# Supported helm backends

This project comes with support for HTTPS, OCI, and [S3](./internal/helm/s3.go) backends.

# Our personal GitHub actions workflows

The workflow we use is the one below, which creates a pull request using a GitHub Application's token and enables auto
merge on the pull request.  Each step is documented

```yaml
name: Auto update helm files
on:
  # Allow other workflows, which build helm charts, to trigger this workflow as a push event on new chart pushes
  workflow_dispatch:
  # Catch up daily
  schedule:
    - cron: "0 0 * * *"
jobs:
  plantrigger:
    runs-on: ubuntu-latest
    name: Force update of helm versions
    steps:
      # Use a github application for our token.  You'll need to make the application and public a private key PEM as a secret
      - name: Generate token
        id: generate_token
        uses: peter-murray/workflow-application-token-action@v1
        with:
          application_id: ${{ secrets.APP_ID }}
          application_private_key: ${{ secrets.APP_PEM }}
      - name: Checkout
        uses: actions/checkout@v2
      # We use S3, so also configure AWS credentials to read the S3 bucket
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v1
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-west-2
          role-duration-seconds: 1200
      # Do the helm updates
      - name: update helm
        uses: cresta/helm-autoupdate@v1.7
      # Only make a PR if there are changes
      - name: check for changes
        id: changes
        run: |
          if [[ `git status --porcelain` ]]; then
            echo '::set-output name=CHANGES::true'
          else
            echo '::set-output name=CHANGES::false'
          fi
      # Create the pull request (notice the if statement)
      - name: Create PR to terraform repo
        uses: peter-evans/create-pull-request@v3
        id: cpr
        if: steps.changes.outputs.CHANGES == 'true'
        with:
          token: ${{ steps.generate_token.outputs.token }}
          branch: helm-autoupdate
          delete-branch: true
          title: "Forced helm auto update"
          labels: forced-workflow
          committer: Forced Replan <noreply@cresta.ai>
          body: "A forced auto update of helm versions"
          commit-message: "A forced auto update of helm versions"
      # Enable auto merge on the PR.  This part requires the generated token above
      - name: Enable Pull Request Auto Merge
        if: steps.cpr.outputs.pull-request-operation == 'created'
        uses: peter-evans/enable-pull-request-automerge@v2
        with:
          token: ${{ steps.generate_token.outputs.token }}
          pull-request-number: ${{ steps.cpr.outputs.pull-request-number }}
          merge-method: squash
```

This workflow allows itself to be triggered by other workflows.  In the repositories that create helm charts, they will
run an action like this.

```yaml
name: Build Project

on: push

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v2
      # Setup AWS for chart upload
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v1
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-west-2
      # Build charts on every push
      - name: Build charts
        run: ./make.sh github_actions_lint_build_charts
      # Only upload charts on the master branch
      - name: Build and Push charts
        if: github.ref == 'refs/heads/master'
        run: ./make.sh github_actions_upload_charts
      # Trigger an automatic update for helm versions
      - name: Tell helm auto update there may be a new helm version
        if: github.ref == 'refs/heads/master'
        run: gh -R cresta/flux2 workflow run helm-autoupdate.yml
        env:
          # Note: there are some bugs with application GH tokens that don't allow them
          #       to dispatch workflows :(
          # You need to use your personal token here
          GH_TOKEN: ${{ secrets.GITHUB_PAT }}

```

# Usage

For a simple example, see the workflow file in [helm-autoupdate-testing](https://github.com/cresta/helm-autoupdate-testing/blob/main/.github/workflows/update-helm-versions.yaml)
