FROM golang:1.26

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download 

COPY ./cmd/ ./cmd
COPY ./pkg/ ./pkg

RUN GOOS=linux go build cmd/main.go 

CMD ["./main"]
