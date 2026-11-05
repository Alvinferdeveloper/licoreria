# Builds with both `podman build` and `docker build`.
FROM docker.io/library/golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM docker.io/library/alpine:3.22
# postgresql16-client provides pg_dump/pg_restore for the backups module.
RUN apk add --no-cache ca-certificates tzdata postgresql16-client \
  && adduser -D -H -u 10001 app

ENV TZ=America/Managua \
    HTTP_ADDR=:8080
COPY --from=build /out/api /usr/local/bin/api

USER app
EXPOSE 8080
ENTRYPOINT ["api"]
CMD ["serve"]
