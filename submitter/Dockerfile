FROM golang:1.23-alpine AS build

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN go build -o submitter main.go

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/submitter .

EXPOSE 8000

CMD ["./submitter"]