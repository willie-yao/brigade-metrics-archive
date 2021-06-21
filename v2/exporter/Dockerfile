FROM brigadecore/go-tools:v0.1.0
ARG VERSION
ARG COMMIT
ENV CGO_ENABLED=0
# WORKDIR /src
# COPY sdk/ sdk/
WORKDIR /src/v2
COPY v2/exporter/ exporter/
COPY v2/go.mod go.mod
COPY v2/go.sum go.sum

RUN go build \
  -o ../bin/exporter \
  -ldflags "-w -X github.com/willie-yao/brigade-prometheus/v2/exporter/internal/version.version=$VERSION -X github.com/willie-yao/brigade-prometheus/v2/exporter/internal/version.commit=$COMMIT" \
  ./exporter

EXPOSE 80

FROM scratch
COPY --from=0 /src/bin/ /brigade-prometheus/bin/
ENTRYPOINT ["/brigade-prometheus/bin/exporter"]
