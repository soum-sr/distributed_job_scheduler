FROM golang:1.24-alpine AS build

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN go build -o coordinator .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/coordinator .

EXPOSE 9000

CMD ["./coordinator"]