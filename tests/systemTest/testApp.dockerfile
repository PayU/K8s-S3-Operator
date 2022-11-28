FROM golang:latest as builder

WORKDIR /app

COPY test_app.go ./

COPY go.mod go.mod

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Command to run the executable
CMD ["./main"] 