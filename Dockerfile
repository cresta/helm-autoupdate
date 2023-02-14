FROM golang:1.20.1-alpine as build

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' -o /helm-autoupdate ./cmd/helm-autoupdate/main.go

FROM scratch
COPY --from=build /helm-autoupdate /helm-autoupdate
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/helm-autoupdate"]
