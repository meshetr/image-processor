FROM golang:1.15-alpine AS build
RUN apk update && apk add --no-cache ca-certificates tzdata && update-ca-certificates
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app .

FROM scratch
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app /
EXPOSE 50051
ENTRYPOINT ["/app"]