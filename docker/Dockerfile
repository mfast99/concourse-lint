# The container image of concourse-lint, built by GoReleaser (dockers_v2 in
# .goreleaser.yaml) from the release binaries; `docker build .` does not
# work, use `make release-snapshot`.
#
# Alpine instead of scratch or distroless: CI systems such as GitLab CI and
# container jobs in GitHub Actions run their scripts with sh in the image.
# The digest pins the base image; Dependabot proposes new ones.
FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# GoReleaser puts the binary of each platform into a folder named after it
# (linux/amd64/, linux/arm64/).
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/concourse-lint /usr/local/bin/concourse-lint

# The license texts, as in the release archives.
COPY LICENSE NOTICE /usr/share/doc/concourse-lint/
COPY cmd/concourse-lint/third_party_licenses.txt /usr/share/doc/concourse-lint/THIRD_PARTY_LICENSES.txt

# nobody: the linter only reads files and needs no rights of its own.
USER 65534:65534
WORKDIR /work
ENTRYPOINT ["concourse-lint"]
CMD ["--help"]
