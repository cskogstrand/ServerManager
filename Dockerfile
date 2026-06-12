FROM golang:alpine AS build
RUN apk --no-cache add gcc g++ make git npm p7zip musl-dev
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /go/src/app
COPY . .
RUN make deps && make build

# Runtime must be glibc-based: AssettoServer (the optional engine) is a glibc
# .NET binary and cannot run on musl/Alpine. The Server Manager binary is
# statically linked (see Makefile), so it runs here unchanged.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        tzdata \
        p7zip-full \
        libicu72 \
        libssl3 \
        libstdc++6 \
        zlib1g \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /usr/bin
COPY --from=build /go/src/app/bin/sm_linux /usr/local/bin/sm
EXPOSE 3030
ENTRYPOINT ["/usr/local/bin/sm", "-p", "/appdata"]
