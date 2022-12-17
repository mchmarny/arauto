# artomator (Artifact Registry Automator, naming is hard)


[![Go Report Card](https://goreportcard.com/badge/github.com/mchmarny/artomator)](https://goreportcard.com/report/github.com/mchmarny/artomator) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/mchmarny/artomator) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/gojp/goreportcard/blob/master/LICENSE)

[Artifact Registry (AR)](https://cloud.google.com/artifact-registry) `artomator` automates the creation of [Software Bill of Materials (SBOM)](https://www.cisa.gov/sbom), and vulnerability scanning of container images. When deployed in your GCP project, `artomator` will automatically process any images that have the expected [label](https://docs.docker.com/config/labels-custom-metadata/). For example:

```shell
docker build -t $IMAGE_TAG --label artomator-sbom=spdx --label artomator-vuln=true .
```

Adding the `artomator-sbom=spdx` label flag to your existing `docker build` commend will automatically tell `artomator` to add SBOM attestations in SPDX format to to that image (the supported formats are: `cyclonedx` or `spdx`). Additionally, if you also include the `artomator-vuln=true` label, `artomator` will generate a vulnerability report from that SBOM. 

![](images/reg.png)

## how it works

![](images/flow.png)

1. Whenever an image is published to the Artifact Registry 
2. A [registry event](https://cloud.google.com/artifact-registry/docs/configure-notifications) is automatically published if there is a [PubSub](https://cloud.google.com/pubsub/docs/overview) topic named `gcr` in the same project
3. PubSub subscription pushes that event to `artomator` service in [Cloud Run](https://cloud.google.com/run) with the operation type (e.g. `INSERT`) and the image digest (SHA256)
4. The `artomator` service retrieves metadata for that image from the registry
5. Signs that image using KMS key and creates the requested artifacts (SBOM or vulnerability report) based on the labels
6. Creates attestation for these artifacts on the original image using the KMS key, and pushes it all the registry
7. (optional) If GCS bucket is configured, `artomator` will persist the generated artifacts
8. Store the processed image digests in Redis to avoid re-processing the same image (technically adding attestation to an image creates yet another event so this could cause recursion without that check)

To processes images, `artomator` uses:

* [cosign](https://github.com/sigstore/cosign) for image signing and verification
* [syft](https://github.com/anchore/syft) for SBOM generation 
* [grype](https://github.com/anchore/grype) for vulnerability scans 
* [jq](https://stedolan.github.io/jq/) for JSON operations 

## artifacts 

The artifacts saved in GCS are all prefixed with the SHA from the image digest. For example, if the image digest was:

```shell
us-west1-docker.pkg.dev/cloudy-demos/artomator/tester@sha256:acaccb6c8f975ee7df7f46468fae28fb5014cf02c2835d2dc37bf6961e648838
```

then the list of artifacts in the registry for that image will be: 

* acaccb6c8f975ee7df7f46468fae28fb5014cf02c2835d2dc37bf6961e648838-sbom.json
* acaccb6c8f975ee7df7f46468fae28fb5014cf02c2835d2dc37bf6961e648838-vuln.json
* acaccb6c8f975ee7df7f46468fae28fb5014cf02c2835d2dc37bf6961e648838-meta.json

where:

* `-sbom.json` is SPDX 2.3 formatted SBOM file
* `-vuln.json` is the vulnerability report based on the SBOM based on `grype` DB
* `-meta.json` is the image metadata in the registry as it was when the image was processed

## deployment 

To deploy the prebuilt `artomator` image with all the dependencies run:

> Note, provisioning Redis service may take a few minutes

```shell
make deploy
```

This will:

* Enable required APIs
* Create artifact registry (`artomator`)
* Configure KMS key (`keyRings/artomator/cryptoKeys/artomator-signer`)
* PubSub topic (`gcr`) and subscription to that topic (`gcr-sub`)
* Deploy Cloud Run service (`artomator`) to process the registry events

## build your own

To build the `artomator` image yourself in your own project, first, enable required APIs, create registry, and configure KMS key:

```shell
tools/setup
```

Then, build the `artomator` image locally, sign it, generate its own SBOM with vulnerability report, publish that image to your own registry (created in setup), and run attestation validation to make sure the image is ready for use:

```shell
tools/build
```

Finally, create the PubSub topic with push subscription, and deploy Cloud Run service to process the registry events: 

> Note, the created Cloud Run service requires `roles/run.invoker` roles so only the PubSub push subscription will be allowed to invoke that service. 

```shell
tools/deploy
```

## test deployment

To test the deployed `artomator`, use the provided ["hello" Dockerfile](tests/Dockerfile). To build it with both labels (`artomator-sbom=true` and `artomator-vuln=true`) and deploy it: 

```shell
make image
```

## verify processed image

To verify the attestation for `artomator` processed images you will need the KMS key name that was used to sign that image. You retrieve it using the following command:

```shell
export SIGN_KEY=$(gcloud kms keys describe artomator-signer \
  --project $PROJECT_ID \
  --location $REGION \
  --keyring artomator \
  --format json | jq --raw-output '.name')
```

You can check the key like this: 

```shell
echo $SIGN_KEY
```

It should look something like this

```shell
projects/$PROJECT_ID/locations/$REGION/keyRings/artomator/cryptoKeys/artomator-signer/cryptoKeyVersions/1
```

Once have the signing key, you can verify any image that was processed by `artomator` like this:

> Note, the `$IMAGE_DIGEST` has to be the fully qualified image URI with the SHA. For example `us-west1-docker.pkg.dev/cloudy-demos/artomator/tester@sha256:59d5b8eb5525307dde52aa51382676e74240bb79eb92a67a1f2a760382a21d78`

```shell
cosign verify-attestation --type=spdx --key "gcpkms://${SIGN_KEY}" $IMAGE_DIGEST \
    | jq -r .payload | base64 -d | jq -r .predicateType
```

> Note, you can check the attestation for either of the two types that `artomator` creates by changing the `--type` flag in the above command to either `spdx` (SBOM), `vuln` which is the vulnerability report

The result should look something like this: 

```shell
Verification for us-west1-docker.pkg.dev/cloudy-demos/artomator/tester@sha256:59d5b8eb5525307dde52aa51382676e74240bb79eb92a67a1f2a760382a21d78 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
https://spdx.dev/Document
```

To save any of these artifacts locally: 

```shell
cosign verify-attestation --type=spdx --key "gcpkms://${SIGN_KEY}" $IMAGE_DIGEST \
    | jq -r .payload | base64 -d > sbom.spdx.json
```

## cleanup

To delete all created resources run: 

```shell
make clean
```

> Note, this does not remove the created KMS resources. 

## disclaimer

This is my personal project and it does not represent my employer. While I do my best to ensure that everything works, I take no responsibility for issues caused by this code.