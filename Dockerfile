FROM golang:alpine AS build
RUN apk --no-cache add gcc g++ make git npm p7zip
ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /go/src/app
COPY . .
RUN make deps && make build

FROM alpine:3.17
RUN apk --no-cache add ca-certificates p7zip
WORKDIR /usr/bin
COPY --from=build /go/src/app/bin/sm_linux /usr/local/bin/sm
EXPOSE 3030
ENTRYPOINT ["/usr/local/bin/sm", "-p", "/appdata"]
