FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/cuevasysimas ./cmd/cuevasysimas \
 && CGO_ENABLED=0 go build -o /out/issuer ./cmd/issuer

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
COPY --from=build /out/cuevasysimas /usr/local/bin/cuevasysimas
COPY --from=build /out/issuer /usr/local/bin/issuer
EXPOSE 64711
ENTRYPOINT ["cuevasysimas"]
