# ---- build stage ----
FROM golang:1.22-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/internal/server

# ---- run stage ----
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/app /app/app

ENV PORT=8080
EXPOSE 8080
CMD ["/app/app"]
