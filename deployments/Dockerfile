# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
COPY --from=build /src/migrations /migrations

EXPOSE 8443
ENTRYPOINT ["/server"]
