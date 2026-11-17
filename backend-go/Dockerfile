FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download
COPY backend-go/ .
RUN CGO_ENABLED=0 go build -o /kinkit .

FROM alpine:3.20
RUN apk add --no-cache tzdata
ENV TZ=UTC
WORKDIR /app
COPY --from=build /kinkit /app/kinkit
COPY react/dist /app/react_dist
ENV DATA_DIR=/app/data
RUN mkdir -p /app/data/receipts
CMD ["/app/kinkit"]
