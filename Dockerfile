FROM golang:alpine AS build
RUN apk --no-cache add gcc g++ make git npm
WORKDIR /go/src/app
COPY . .
RUN make deps && make build

FROM alpine:3.17
RUN apk --no-cache add ca-certificates
WORKDIR /usr/bin
COPY --from=build /go/src/app/bin/sm /usr/local/bin/sm
EXPOSE 3030
ENTRYPOINT ["/usr/local/bin/sm", "-p", "/appdata"]
