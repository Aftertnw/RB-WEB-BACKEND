# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/app ./app

EXPOSE 8080
ENV PORT=8080
CMD ["./app"]
