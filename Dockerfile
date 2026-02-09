ARG BASE_IMAGE=mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20260107@sha256:138fe2905465e384b232ffe8ba3147de04c633a83f29d8df00d6817e3eacb0d2

FROM ${BASE_IMAGE}
WORKDIR /
COPY bin/manager .
COPY bin/clustergate .
USER 65532:65532
ENTRYPOINT ["/manager"]