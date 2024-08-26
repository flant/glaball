FROM golang:1.21-bullseye@sha256:40a67e6626bead90d5c7957bd0354cfeb8400e61acc3adc256e03252630014a6 as builder

ARG versionflags

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 go build -v -a -tags netgo -ldflags="-extldflags '-static' -s -w $versionflags" -o build/glaball *.go


FROM debian:bullseye-slim@sha256:9058862a1be84689bd13292549ba981364f85ff99e50a612f94b188ac69db137

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -qy --no-install-recommends \
        ca-certificates

COPY --from=builder /src/build/glaball /usr/local/bin/glaball

CMD [ "/usr/local/bin/glaball" ]
