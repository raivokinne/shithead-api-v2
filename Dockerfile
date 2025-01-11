FROM golang:1.23-alpine AS build
RUN apk add --no-cache git bash make
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go install github.com/air-verse/air@latest

FROM golang:1.23-alpine AS dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN apk add --no-cache bash git make
RUN go install github.com/air-verse/air@latest
EXPOSE 8080
CMD ["air"]
